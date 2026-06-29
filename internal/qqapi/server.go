package qqapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"robot_yysls/internal/flow"
	"robot_yysls/internal/style"
)

const EventGroupAtMessageCreate = "GROUP_AT_MESSAGE_CREATE"

type GroupReplySender interface {
	SendGroupReply(ctx context.Context, groupOpenID, content, eventID, msgID string, msgSeq int) error
	SendGroupTemplateFile(ctx context.Context, groupOpenID, templatePath, eventID, msgID string, msgSeq int) error
}

type WebhookServer struct {
	BotSecret string
	Flow      *flow.Coordinator
	Sender    GroupReplySender
	Now       func() time.Time
}

type GroupAtMessageData struct {
	ID           string `json:"id"`
	GroupOpenID  string `json:"group_openid"`
	MemberOpenID string `json:"member_openid"`
	Content      string `json:"content"`
	EventID      string `json:"event_id"`
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

	reply, templates, err := s.dispatchGroupText(event)
	if err != nil || reply == "" || s.Sender == nil {
		return err
	}
	if err := s.Sender.SendGroupReply(ctx, event.GroupOpenID, reply, event.EventID, event.ID, 1); err != nil {
		return err
	}
	for i, template := range templates {
		if err := s.Sender.SendGroupTemplateFile(ctx, event.GroupOpenID, template.TemplatePath, event.EventID, event.ID, i+2); err != nil {
			return err
		}
	}
	return nil
}

func (s *WebhookServer) dispatchGroupText(event GroupAtMessageData) (string, []style.Config, error) {
	if s.Flow == nil {
		return "", nil, nil
	}
	text := normalizeAtContent(event.Content)
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}

	switch {
	case strings.Contains(text, "计算毕业率"):
		reply, err := s.Flow.Start(event.GroupOpenID, event.MemberOpenID, now)
		return reply, nil, err
	default:
		selection, err := s.Flow.SelectStyles(event.GroupOpenID, event.MemberOpenID, text, now)
		return selection.Reply, selection.Styles, err
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
