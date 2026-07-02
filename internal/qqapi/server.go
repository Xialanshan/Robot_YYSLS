package qqapi

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"robot_yysls/internal/flow"
)

const EventGroupAtMessageCreate = "GROUP_AT_MESSAGE_CREATE"
const ocrEventDedupTTL = 10 * time.Minute

type GroupReplySender interface {
	SendGroupReply(ctx context.Context, groupOpenID, content, eventID, msgID string, msgSeq int) error
	SendGroupTemplateFile(ctx context.Context, groupOpenID, templatePath, eventID, msgID string, msgSeq int) error
}

type WebhookServer struct {
	BotSecret string
	Flow      *flow.Coordinator
	OCR       OCRCommandHandler
	Sender    GroupReplySender
	Now       func() time.Time

	mu              sync.Mutex
	ocrEventHandled map[string]time.Time
}

type GroupAtMessageData struct {
	ID           string             `json:"id"`
	GroupOpenID  string             `json:"group_openid"`
	Author       GroupMessageAuthor `json:"author"`
	MemberOpenID string             `json:"member_openid"`
	Content      string             `json:"content"`
	EventID      string             `json:"event_id"`
	Attachments  []MessageMedia     `json:"attachments"`
	Images       []MessageMedia     `json:"images"`
	Image        *MessageMedia      `json:"image"`
}

type GroupMessageAuthor struct {
	MemberOpenID string `json:"member_openid"`
}

type MessageMedia struct {
	ID          string `json:"id,omitempty"`
	FileID      string `json:"file_id,omitempty"`
	FileUUID    string `json:"file_uuid,omitempty"`
	URL         string `json:"url,omitempty"`
	FileURL     string `json:"file_url,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
	Filename    string `json:"filename,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Size        int64  `json:"size,omitempty"`
}

type ImageReference struct {
	URL      string
	FileID   string
	Filename string
	Size     int64
}

func (d GroupAtMessageData) MemberID() string {
	if d.Author.MemberOpenID != "" {
		return d.Author.MemberOpenID
	}
	return d.MemberOpenID
}

func (d GroupAtMessageData) ImageReferences() []ImageReference {
	refs := make([]ImageReference, 0, len(d.Attachments)+len(d.Images)+1)
	for _, media := range d.Attachments {
		refs = appendImageReference(refs, media)
	}
	for _, media := range d.Images {
		refs = appendImageReference(refs, media)
	}
	if d.Image != nil {
		refs = appendImageReference(refs, *d.Image)
	}
	return refs
}

func appendImageReference(refs []ImageReference, media MessageMedia) []ImageReference {
	url := firstNonEmpty(media.URL, media.FileURL, media.DownloadURL)
	fileID := firstNonEmpty(media.FileID, media.FileUUID, media.ID)
	filename := firstNonEmpty(media.Filename, media.FileName)
	if url == "" && fileID == "" && filename == "" {
		return refs
	}
	return append(refs, ImageReference{
		URL:      url,
		FileID:   fileID,
		Filename: filename,
		Size:     media.Size,
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (s *WebhookServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	if !VerifyWebhookSignature(s.BotSecret, r.Header.Get("X-Signature-Timestamp"), body, r.Header.Get("X-Signature-Ed25519")) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var payload Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// #region debug-point
	log.Printf("debug_webhook_received op=%d type=%q payload_id=%q body_bytes=%d", payload.Op, payload.T, payload.ID, len(body))
	// #endregion debug-point

	w.Header().Set("Content-Type", "application/json")
	if payload.Op == OpHTTPCallbackVerify {
		resp, err := BuildValidationResponse(s.BotSecret, payload)
		if err != nil {
			http.Error(w, "invalid validation payload", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	if payload.Op == OpDispatch && payload.T == EventGroupAtMessageCreate {
		if err := s.handleGroupAtMessage(payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	_ = json.NewEncoder(w).Encode(CallbackACK())
}

func (s *WebhookServer) handleGroupAtMessage(payload Payload) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	startedAt := time.Now()

	var event GroupAtMessageData
	if err := json.Unmarshal(payload.D, &event); err != nil {
		return err
	}
	if event.EventID == "" {
		event.EventID = payload.ID
	}

	memberID := event.MemberID()
	// #region debug-point
	log.Printf("debug_group_message_start group=%q member=%q msg=%q event=%q started_at=%s", event.GroupOpenID, memberID, event.ID, event.EventID, startedAt.Format(time.RFC3339Nano))
	// #endregion debug-point
	text := normalizeAtContent(event.Content)
	if strings.Contains(text, "OCR计算") {
		return s.handleOCRAsync(event, memberID, text, startedAt)
	}
	reply, templates, err := s.dispatchGroupText(ctx, event, memberID)
	imageRefs := event.ImageReferences()
	log.Printf("group_at_message group=%q member=%q msg=%q event=%q raw_content=%q images=%d image_summary=%s reply_empty=%t templates=%d err=%v", event.GroupOpenID, memberID, event.ID, event.EventID, event.Content, len(imageRefs), summarizeImageReferences(imageRefs), reply == "", len(templates), err)
	// #region debug-point
	log.Printf("debug_group_message_after_dispatch group=%q member=%q msg=%q event=%q duration_ms=%d reply_empty=%t templates=%d err=%v", event.GroupOpenID, memberID, event.ID, event.EventID, time.Since(startedAt).Milliseconds(), reply == "", len(templates), err)
	// #endregion debug-point
	if err != nil {
		return err
	}
	if reply == "" || s.Sender == nil {
		return nil
	}
	// #region debug-point
	log.Printf("debug_group_reply_attempt group=%q msg=%q event=%q msg_seq=%d content_preview=%q", event.GroupOpenID, event.ID, event.EventID, 1, truncateForDebug(reply, 120))
	// #endregion debug-point
	if err := s.Sender.SendGroupReply(ctx, event.GroupOpenID, reply, event.EventID, event.ID, 1); err != nil {
		log.Printf("send_reply_failed group=%q msg=%q event=%q err=%v", event.GroupOpenID, event.ID, event.EventID, err)
	}
	for i, template := range templates {
		log.Printf("send_template index=%d name=%q path=%q msg_seq=%d", i, template.Name, template.Path, i+2)
		if err := s.Sender.SendGroupTemplateFile(ctx, event.GroupOpenID, template.Path, event.EventID, event.ID, i+2); err != nil {
			log.Printf("send_template_failed index=%d name=%q path=%q err=%v", i, template.Name, template.Path, err)
			continue
		}
		log.Printf("send_template_ok index=%d name=%q path=%q", i, template.Name, template.Path)
	}
	return nil
}

func (s *WebhookServer) handleOCRAsync(event GroupAtMessageData, memberID, text string, startedAt time.Time) error {
	if s.OCR == nil {
		return s.sendImmediateOCRReply(event, "OCR 功能尚未启用，请稍后再试。")
	}
	if len(event.ImageReferences()) == 0 {
		return s.sendImmediateOCRReply(event, "请在同一条 @ 消息里附带截图，例如：@机器人 OCR计算 鸣金虹 + 2 张属性截图。")
	}

	if !s.tryStartOCREvent(event.EventID, s.currentTime()) {
		// #region debug-point
		log.Printf("debug_ocr_duplicate_event_ignored group=%q member=%q msg=%q event=%q", event.GroupOpenID, memberID, event.ID, event.EventID)
		// #endregion debug-point
		return nil
	}

	if err := s.sendImmediateOCRReply(event, "已收到截图，正在识别并计算，请稍候。"); err != nil {
		log.Printf("send_reply_failed group=%q msg=%q event=%q err=%v", event.GroupOpenID, event.ID, event.EventID, err)
	}

	go s.runOCRTask(event, memberID, text, startedAt)
	return nil
}

func (s *WebhookServer) runOCRTask(event GroupAtMessageData, memberID, text string, startedAt time.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	now := s.currentTime()
	imageRefs := event.ImageReferences()

	// #region debug-point
	log.Printf("debug_ocr_task_started group=%q member=%q msg=%q event=%q images=%d", event.GroupOpenID, memberID, event.ID, event.EventID, len(imageRefs))
	// #endregion debug-point
	result, err := s.OCR.Handle(ctx, event.GroupOpenID, memberID, text, imageRefs, now)
	log.Printf("group_at_message group=%q member=%q msg=%q event=%q raw_content=%q images=%d image_summary=%s reply_empty=%t templates=%d err=%v", event.GroupOpenID, memberID, event.ID, event.EventID, event.Content, len(imageRefs), summarizeImageReferences(imageRefs), result.Reply == "", len(result.Templates), err)
	// #region debug-point
	log.Printf("debug_ocr_task_finished group=%q member=%q msg=%q event=%q duration_ms=%d handled=%t reply_empty=%t templates=%d err=%v", event.GroupOpenID, memberID, event.ID, event.EventID, time.Since(startedAt).Milliseconds(), result.Handled, result.Reply == "", len(result.Templates), err)
	// #endregion debug-point

	reply := result.Reply
	if err != nil {
		reply = "OCR 识别失败，请稍后重试。错误：" + err.Error()
	}
	if reply == "" || s.Sender == nil {
		return
	}

	// #region debug-point
	log.Printf("debug_ocr_task_reply_attempt group=%q msg=%q event=%q content_preview=%q", event.GroupOpenID, event.ID, event.EventID, truncateForDebug(reply, 120))
	// #endregion debug-point
	if err := s.Sender.SendGroupReply(context.Background(), event.GroupOpenID, reply, "", "", 0); err != nil {
		log.Printf("send_reply_failed group=%q msg=%q event=%q err=%v", event.GroupOpenID, event.ID, event.EventID, err)
	}
}

func (s *WebhookServer) sendImmediateOCRReply(event GroupAtMessageData, reply string) error {
	if reply == "" || s.Sender == nil {
		return nil
	}
	// #region debug-point
	log.Printf("debug_group_reply_attempt group=%q msg=%q event=%q msg_seq=%d content_preview=%q", event.GroupOpenID, event.ID, event.EventID, 1, truncateForDebug(reply, 120))
	// #endregion debug-point
	return s.Sender.SendGroupReply(context.Background(), event.GroupOpenID, reply, event.EventID, event.ID, 1)
}

func (s *WebhookServer) tryStartOCREvent(eventID string, now time.Time) bool {
	if eventID == "" {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ocrEventHandled == nil {
		s.ocrEventHandled = make(map[string]time.Time)
	}
	for id, createdAt := range s.ocrEventHandled {
		if now.Sub(createdAt) > ocrEventDedupTTL {
			delete(s.ocrEventHandled, id)
		}
	}
	if _, exists := s.ocrEventHandled[eventID]; exists {
		return false
	}
	s.ocrEventHandled[eventID] = now
	return true
}

func (s *WebhookServer) currentTime() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func summarizeImageReferences(refs []ImageReference) string {
	if len(refs) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(refs))
	for _, ref := range refs {
		parts = append(parts, "{has_url="+boolString(ref.URL != "")+",has_file_id="+boolString(ref.FileID != "")+",filename="+ref.Filename+"}")
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func (s *WebhookServer) dispatchGroupText(ctx context.Context, event GroupAtMessageData, memberID string) (string, []OutboundTemplate, error) {
	text := normalizeAtContent(event.Content)
	now := s.currentTime()

	switch {
	case strings.Contains(text, "发我模板"):
		if s.OCR == nil {
			return "OCR 功能尚未启用，请稍后再试。", nil, nil
		}
		// #region debug-point
		log.Printf("debug_ocr_dispatch_enter group=%q member=%q msg=%q event=%q text=%q images=%d", event.GroupOpenID, memberID, event.ID, event.EventID, text, len(event.ImageReferences()))
		ocrStartedAt := time.Now()
		// #endregion debug-point
		result, err := s.OCR.Handle(ctx, event.GroupOpenID, memberID, text, event.ImageReferences(), now)
		// #region debug-point
		log.Printf("debug_ocr_dispatch_exit group=%q member=%q msg=%q event=%q duration_ms=%d handled=%t reply_empty=%t templates=%d err=%v", event.GroupOpenID, memberID, event.ID, event.EventID, time.Since(ocrStartedAt).Milliseconds(), result.Handled, result.Reply == "", len(result.Templates), err)
		// #endregion debug-point
		return result.Reply, result.Templates, err
	case strings.Contains(text, "计算毕业率"):
		if s.Flow == nil {
			return "", nil, nil
		}
		reply, err := s.Flow.Start(event.GroupOpenID, memberID, now)
		return reply, nil, err
	default:
		if s.Flow == nil {
			return "", nil, nil
		}
		selection, err := s.Flow.SelectStyles(event.GroupOpenID, memberID, text, now)
		templates := make([]OutboundTemplate, 0, len(selection.Styles))
		for _, cfg := range selection.Styles {
			templates = append(templates, OutboundTemplate{Name: cfg.Name, Path: cfg.TemplatePath})
		}
		return selection.Reply, templates, err
	}
}

func truncateForDebug(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max] + "..."
}

func normalizeAtContent(content string) string {
	content = strings.TrimSpace(content)
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	fields := strings.Fields(content)
	cleaned := make([]string, 0, len(fields))
	for _, field := range fields {
		if strings.HasPrefix(field, "<@") && strings.HasSuffix(field, ">") {
			continue
		}
		cleaned = append(cleaned, field)
	}
	if strings.Contains(content, "\n") {
		return strings.TrimSpace(strings.Join(cleaned, "\n"))
	}
	return strings.TrimSpace(strings.Join(cleaned, " "))
}
