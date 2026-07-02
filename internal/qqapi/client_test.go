package qqapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if req["appId"] != "1904844772" || req["clientSecret"] != "secret" {
			t.Fatalf("request = %+v", req)
		}
		_, _ = w.Write([]byte(`{"access_token":"token","expires_in":"7200"}`))
	}))
	defer server.Close()

	client := NewClient("1904844772", "secret", server.URL, server.URL+"/token", WithHTTPClient(server.Client()))
	token, err := client.GetAccessToken(context.Background())
	if err != nil {
		t.Fatalf("GetAccessToken() error = %v", err)
	}
	if token.Token != "token" || token.ExpiresIn != 7200 {
		t.Fatalf("token = %+v", token)
	}
}

func TestSendGroupText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/groups/group-openid/messages" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "QQBot token" {
			t.Fatalf("Authorization = %q", got)
		}
		var req SendGroupMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if req.Content != "@user hello" || req.MsgType != MessageTypeText || req.EventID != "event-id" || req.MsgID != "msg-id" || req.MsgSeq != 2 {
			t.Fatalf("request = %+v", req)
		}
		_, _ = w.Write([]byte(`{"id":"message-id","timestamp":"123"}`))
	}))
	defer server.Close()

	client := NewClient("appid", "secret", server.URL, server.URL+"/token", WithHTTPClient(server.Client()))
	resp, err := client.SendGroupText(context.Background(), "token", "group-openid", "@user hello", "event-id", "msg-id", 2)
	if err != nil {
		t.Fatalf("SendGroupText() error = %v", err)
	}
	if resp.ID != "message-id" || resp.Timestamp != 123 {
		t.Fatalf("response = %+v", resp)
	}
}

func TestSendGroupTextAcceptsRFC3339Timestamp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"message-id","timestamp":"2026-07-01T15:21:46+08:00"}`))
	}))
	defer server.Close()

	client := NewClient("appid", "secret", server.URL, server.URL+"/token", WithHTTPClient(server.Client()))
	resp, err := client.SendGroupText(context.Background(), "token", "group-openid", "hello", "event-id", "msg-id", 1)
	if err != nil {
		t.Fatalf("SendGroupText() error = %v", err)
	}
	if resp.ID != "message-id" || resp.Timestamp == 0 {
		t.Fatalf("response = %+v", resp)
	}
}

func TestSendGroupTextAllowsActivePushWithoutReplyIdentifiers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req SendGroupMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if req.Content != "hello" || req.MsgType != MessageTypeText {
			t.Fatalf("request = %+v", req)
		}
		if req.EventID != "" || req.MsgID != "" || req.MsgSeq != 0 {
			t.Fatalf("request should omit reply identifiers: %+v", req)
		}
		_, _ = w.Write([]byte(`{"id":"message-id","timestamp":"123"}`))
	}))
	defer server.Close()

	client := NewClient("appid", "secret", server.URL, server.URL+"/token", WithHTTPClient(server.Client()))
	resp, err := client.SendGroupText(context.Background(), "token", "group-openid", "hello", "", "", 0)
	if err != nil {
		t.Fatalf("SendGroupText() error = %v", err)
	}
	if resp.ID != "message-id" || resp.Timestamp != 123 {
		t.Fatalf("response = %+v", resp)
	}
}

func TestSendGroupMediaIncludesRequiredContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/groups/group-openid/messages" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var req SendGroupMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if req.Content == "" || req.MsgType != MessageTypeMedia || req.Media == nil || req.Media.FileInfo != "file-info" {
			t.Fatalf("request = %+v", req)
		}
		_, _ = w.Write([]byte(`{"id":"message-id","timestamp":123}`))
	}))
	defer server.Close()

	client := NewClient("appid", "secret", server.URL, server.URL+"/token", WithHTTPClient(server.Client()))
	resp, err := client.SendGroupMedia(context.Background(), "token", "group-openid", "file-info", "event-id", "msg-id", 2)
	if err != nil {
		t.Fatalf("SendGroupMedia() error = %v", err)
	}
	if resp.ID != "message-id" {
		t.Fatalf("response = %+v", resp)
	}
}

func TestUploadGroupFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/groups/group-openid/files" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var req UploadGroupFileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if req.FileType != FileTypeFile || req.FileName != "template.xlsx" || req.URL != "https://example.test/template.xlsx" || req.SrvSendMsg == nil || *req.SrvSendMsg {
			t.Fatalf("request = %+v", req)
		}
		_, _ = w.Write([]byte(`{"file_uuid":"file-id","file_info":"file-info","ttl":3600}`))
	}))
	defer server.Close()

	client := NewClient("appid", "secret", server.URL, server.URL+"/token", WithHTTPClient(server.Client()))
	resp, err := client.UploadGroupFile(context.Background(), "token", "group-openid", "template.xlsx", "https://example.test/template.xlsx", "")
	if err != nil {
		t.Fatalf("UploadGroupFile() error = %v", err)
	}
	if resp.FileInfo != "file-info" {
		t.Fatalf("response = %+v", resp)
	}
}
