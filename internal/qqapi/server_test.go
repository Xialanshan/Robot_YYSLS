package qqapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	if strings.Contains(sender.content, "@member-openid") || !strings.Contains(sender.content, "请选择流派") {
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

func TestWebhookServerOCRCommandDoesNotStartLegacyTemplateFlow(t *testing.T) {
	secret := "secret"
	sender := &fakeReplySender{}
	body := groupAtPayload(t, "event-id", "msg-id", "group-openid", "member-openid", "<@bot> OCR计算 牵丝玉")

	rec := httptest.NewRecorder()
	(&WebhookServer{
		BotSecret: secret,
		Flow:      testFlow(t),
		Sender:    sender,
		Now:       func() time.Time { return time.Date(2026, 6, 29, 10, 0, 0, 0, time.UTC) },
	}).ServeHTTP(rec, signedRequest(t, secret, body))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(sender.content, "已识别 OCR 计算指令") {
		t.Fatalf("OCR command reply = %q", sender.content)
	}
	if strings.Contains(sender.content, "请选择流派") || len(sender.templates) != 0 {
		t.Fatalf("legacy flow unexpectedly triggered: content=%q templates=%+v", sender.content, sender.templates)
	}
}

func TestGroupAtMessageImageReferences(t *testing.T) {
	event := GroupAtMessageData{
		Attachments: []MessageMedia{
			{
				URL:      "https://example.test/a.png",
				FileID:   "file-id",
				Filename: "attrs.png",
				Size:     1024,
			},
		},
	}

	refs := event.ImageReferences()
	if len(refs) != 1 {
		t.Fatalf("refs = %+v, want 1", refs)
	}
	if refs[0].URL == "" || refs[0].FileID != "file-id" || refs[0].Filename != "attrs.png" {
		t.Fatalf("ref = %+v", refs[0])
	}
}

func TestWebhookServerContinuesTemplateSendWhenTextReplyFails(t *testing.T) {
	secret := "secret"
	sender := &fakeReplySender{replyErr: errors.New("reply failed")}
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
	if len(sender.templates) != 1 || sender.templates[0] != "template.xlsx" {
		t.Fatalf("templates = %+v", sender.templates)
	}
}

func TestWebhookServerACKsWhenTemplateSendFails(t *testing.T) {
	secret := "secret"
	sender := &fakeReplySender{templateErr: errors.New("template failed")}
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
	if len(sender.templates) != 1 {
		t.Fatalf("template send attempts = %+v, want 1", sender.templates)
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
	replyErr    error
	templateErr error
}

func (s *fakeReplySender) SendGroupReply(_ context.Context, groupOpenID, content, eventID, msgID string, msgSeq int) error {
	s.groupOpenID = groupOpenID
	s.content = content
	s.eventID = eventID
	s.msgID = msgID
	s.msgSeq = msgSeq
	return s.replyErr
}

func (s *fakeReplySender) SendGroupTemplateFile(_ context.Context, groupOpenID, templatePath, eventID, msgID string, msgSeq int) error {
	s.groupOpenID = groupOpenID
	s.eventID = eventID
	s.msgID = msgID
	s.msgSeq = msgSeq
	s.templates = append(s.templates, templatePath)
	return s.templateErr
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
