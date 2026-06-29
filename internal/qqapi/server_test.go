package qqapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"robot_yysls/internal/flow"
	"robot_yysls/internal/session"
	"robot_yysls/internal/style"
)

func TestWebhookServerValidation(t *testing.T) {
	secret := "secret"
	data, err := json.Marshal(ValidationRequest{PlainToken: "plain", EventTS: "12345"})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	body, err := json.Marshal(Payload{Op: OpHTTPCallbackVerify, D: data})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	req := signedRequest(t, secret, body)
	rec := httptest.NewRecorder()
	(&WebhookServer{BotSecret: secret}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var resp ValidationResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if resp.PlainToken != "plain" || resp.Signature == "" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestWebhookServerGroupAtMessageStartsFlow(t *testing.T) {
	secret := "secret"
	sender := &fakeReplySender{}
	body := groupAtPayload(t, "event-id", "msg-id", "group-openid", "member-openid", "<@bot> 计算毕业率")

	req := signedRequest(t, secret, body)
	rec := httptest.NewRecorder()
	(&WebhookServer{
		BotSecret: secret,
		Flow:      testFlow(t),
		Sender:    sender,
		Now:       func() time.Time { return time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC) },
	}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if sender.groupOpenID != "group-openid" || sender.eventID != "event-id" || sender.msgID != "msg-id" {
		t.Fatalf("sender = %+v", sender)
	}
	if !strings.Contains(sender.content, "@member-openid") || !strings.Contains(sender.content, "请选择流派") {
		t.Fatalf("reply content = %q", sender.content)
	}
}

func TestWebhookServerGroupAtMessageSendsSelectedTemplates(t *testing.T) {
	secret := "secret"
	sender := &fakeReplySender{}
	server := &WebhookServer{
		BotSecret: secret,
		Flow:      testFlow(t),
		Sender:    sender,
		Now:       func() time.Time { return time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC) },
	}

	startBody := groupAtPayload(t, "event-id-1", "msg-id-1", "group-openid", "member-openid", "<@bot> 计算毕业率")
	server.ServeHTTP(httptest.NewRecorder(), signedRequest(t, secret, startBody))

	selectBody := groupAtPayload(t, "event-id-2", "msg-id-2", "group-openid", "member-openid", "<@bot> 牵丝玉")
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, signedRequest(t, secret, selectBody))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(sender.content, "已为你准备以下流派模板") {
		t.Fatalf("reply content = %q", sender.content)
	}
	if len(sender.templates) != 1 || sender.templates[0] != "template.xlsx" {
		t.Fatalf("templates = %+v", sender.templates)
	}
}

func TestWebhookServerRejectsInvalidSignature(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/qq/webhook", bytes.NewReader([]byte(`{"op":12}`)))
	req.Header.Set("X-Signature-Timestamp", "123")
	req.Header.Set("X-Signature-Ed25519", "bad")
	rec := httptest.NewRecorder()

	(&WebhookServer{BotSecret: "secret"}).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

type fakeReplySender struct {
	groupOpenID string
	content     string
	eventID     string
	msgID       string
	msgSeq      int
	templates   []string
}

func (s *fakeReplySender) SendGroupReply(_ context.Context, groupOpenID, content, eventID, msgID string, msgSeq int) error {
	s.groupOpenID = groupOpenID
	s.content = content
	s.eventID = eventID
	s.msgID = msgID
	s.msgSeq = msgSeq
	return nil
}

func (s *fakeReplySender) SendGroupTemplateFile(_ context.Context, groupOpenID, templatePath, eventID, msgID string, msgSeq int) error {
	s.groupOpenID = groupOpenID
	s.eventID = eventID
	s.msgID = msgID
	s.msgSeq = msgSeq
	s.templates = append(s.templates, templatePath)
	return nil
}

func signedRequest(t *testing.T, secret string, body []byte) *http.Request {
	t.Helper()

	timestamp := "1725442341"
	privateKey, err := privateKeyFromSecret(secret)
	if err != nil {
		t.Fatalf("privateKeyFromSecret() error = %v", err)
	}
	var msg bytes.Buffer
	msg.WriteString(timestamp)
	msg.Write(body)
	signature := hex.EncodeToString(ed25519.Sign(privateKey, msg.Bytes()))

	req := httptest.NewRequest(http.MethodPost, "/qq/webhook", bytes.NewReader(body))
	req.Header.Set("X-Signature-Timestamp", timestamp)
	req.Header.Set("X-Signature-Ed25519", signature)
	return req
}

func groupAtPayload(t *testing.T, eventID, msgID, groupOpenID, memberOpenID, content string) []byte {
	t.Helper()

	data, err := json.Marshal(GroupAtMessageData{
		ID:           msgID,
		GroupOpenID:  groupOpenID,
		MemberOpenID: memberOpenID,
		Content:      content,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	body, err := json.Marshal(Payload{
		ID: eventID,
		Op: OpDispatch,
		T:  EventGroupAtMessageCreate,
		D:  data,
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	return body
}

func testFlow(t *testing.T) *flow.Coordinator {
	t.Helper()

	registry, err := style.NewRegistry([]style.Config{
		{ID: "牵丝玉", Name: "牵丝玉", TemplatePath: "template.xlsx", Result: style.CellRef{Sheet: "期望", Cell: "I16"}},
	})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	return &flow.Coordinator{
		Store:  session.NewMemoryStore(),
		Styles: registry,
	}
}
