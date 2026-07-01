package qqapi

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"robot_yysls/internal/flow"
)

const EventGroupAtMessageCreate = "GROUP_AT_MESSAGE_CREATE"

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
		if err := s.handleGroupAtMessage(r.Context(), payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	_ = json.NewEncoder(w).Encode(CallbackACK())
}

func (s *WebhookServer) handleGroupAtMessage(ctx context.Context, payload Payload) error {
	var event GroupAtMessageData
	if err := json.Unmarshal(payload.D, &event); err != nil {
		return err
	}
	if event.EventID == "" {
		event.EventID = payload.ID
	}

	memberID := event.MemberID()
	reply, templates, err := s.dispatchGroupText(ctx, event, memberID)
	imageRefs := event.ImageReferences()
	log.Printf("group_at_message group=%q member=%q msg=%q event=%q raw_content=%q images=%d image_summary=%s reply_empty=%t templates=%d err=%v", event.GroupOpenID, memberID, event.ID, event.EventID, event.Content, len(imageRefs), summarizeImageReferences(imageRefs), reply == "", len(templates), err)
	if err != nil {
		return err
	}
	if reply == "" || s.Sender == nil {
		return nil
	}
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
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}

	switch {
	case strings.Contains(text, "OCR计算"), strings.Contains(text, "发我模板"):
		if s.OCR == nil {
			return "OCR 功能尚未启用，请稍后再试。", nil, nil
		}
		result, err := s.OCR.Handle(ctx, event.GroupOpenID, memberID, text, event.ImageReferences(), now)
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
