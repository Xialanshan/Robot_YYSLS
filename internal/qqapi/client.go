package qqapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	MessageTypeText  = 0
	MessageTypeMedia = 7
	FileTypeFile     = 4
)

type Client struct {
	appID      string
	appSecret  string
	apiBase    string
	tokenURL   string
	httpClient *http.Client
}

type Option func(*Client)

func NewClient(appID, appSecret, apiBase, tokenURL string, opts ...Option) *Client {
	client := &Client{
		appID:      appID,
		appSecret:  appSecret,
		apiBase:    strings.TrimRight(apiBase, "/"),
		tokenURL:   tokenURL,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		if httpClient != nil {
			client.httpClient = httpClient
		}
	}
}

type AccessToken struct {
	Token     string
	ExpiresIn int
}

func (c *Client) GetAccessToken(ctx context.Context) (AccessToken, error) {
	reqBody := map[string]string{
		"appId":        c.appID,
		"clientSecret": c.appSecret,
	}

	var respBody struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in,string"`
	}
	if err := c.doJSON(ctx, http.MethodPost, c.tokenURL, "", reqBody, &respBody); err != nil {
		return AccessToken{}, err
	}
	if respBody.AccessToken == "" {
		return AccessToken{}, fmt.Errorf("empty access token response")
	}
	return AccessToken{Token: respBody.AccessToken, ExpiresIn: respBody.ExpiresIn}, nil
}

type SendGroupMessageRequest struct {
	Content string `json:"content,omitempty"`
	MsgType int    `json:"msg_type"`
	Media   *Media `json:"media,omitempty"`
	EventID string `json:"event_id,omitempty"`
	MsgID   string `json:"msg_id,omitempty"`
	MsgSeq  int    `json:"msg_seq,omitempty"`
}

type Media struct {
	FileInfo string `json:"file_info"`
}

type SendMessageResponse struct {
	ID        string            `json:"id"`
	Timestamp FlexibleTimestamp `json:"timestamp"`
}

type FlexibleTimestamp int64

func (t *FlexibleTimestamp) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	var value int64
	var err error
	if data[0] == '"' {
		unquoted, unquoteErr := strconv.Unquote(string(data))
		if unquoteErr != nil {
			return unquoteErr
		}
		value, err = strconv.ParseInt(unquoted, 10, 64)
	} else {
		value, err = strconv.ParseInt(string(data), 10, 64)
	}
	if err != nil {
		return err
	}
	*t = FlexibleTimestamp(value)
	return nil
}

func (c *Client) SendGroupText(ctx context.Context, accessToken, groupOpenID, content, eventID, msgID string, msgSeq int) (SendMessageResponse, error) {
	req := SendGroupMessageRequest{
		Content: content,
		MsgType: MessageTypeText,
		EventID: eventID,
		MsgID:   msgID,
		MsgSeq:  msgSeq,
	}
	var resp SendMessageResponse
	url := c.apiBase + "/v2/groups/" + groupOpenID + "/messages"
	if err := c.doJSON(ctx, http.MethodPost, url, accessToken, req, &resp); err != nil {
		return SendMessageResponse{}, err
	}
	return resp, nil
}

func (c *Client) SendGroupMedia(ctx context.Context, accessToken, groupOpenID, fileInfo, eventID, msgID string, msgSeq int) (SendMessageResponse, error) {
	req := SendGroupMessageRequest{
		Content: "模板文件",
		MsgType: MessageTypeMedia,
		Media:   &Media{FileInfo: fileInfo},
		EventID: eventID,
		MsgID:   msgID,
		MsgSeq:  msgSeq,
	}
	var resp SendMessageResponse
	url := c.apiBase + "/v2/groups/" + groupOpenID + "/messages"
	if err := c.doJSON(ctx, http.MethodPost, url, accessToken, req, &resp); err != nil {
		return SendMessageResponse{}, err
	}
	return resp, nil
}

type UploadGroupFileRequest struct {
	FileType   int    `json:"file_type"`
	FileName   string `json:"file_name,omitempty"`
	URL        string `json:"url,omitempty"`
	FileData   string `json:"file_data,omitempty"`
	SrvSendMsg *bool  `json:"srv_send_msg,omitempty"`
}

type UploadFileResponse struct {
	FileUUID string `json:"file_uuid"`
	FileInfo string `json:"file_info"`
	TTL      int    `json:"ttl"`
}

func (c *Client) UploadGroupFile(ctx context.Context, accessToken, groupOpenID, fileName, fileURL, fileData string) (UploadFileResponse, error) {
	srvSendMsg := false
	req := UploadGroupFileRequest{
		FileType:   FileTypeFile,
		FileName:   fileName,
		URL:        fileURL,
		FileData:   fileData,
		SrvSendMsg: &srvSendMsg,
	}
	var resp UploadFileResponse
	url := c.apiBase + "/v2/groups/" + groupOpenID + "/files"
	if err := c.doJSON(ctx, http.MethodPost, url, accessToken, req, &resp); err != nil {
		return UploadFileResponse{}, err
	}
	return resp, nil
}

func (c *Client) UploadGroupLocalFile(ctx context.Context, accessToken, groupOpenID, filePath string) (UploadFileResponse, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return UploadFileResponse{}, err
	}
	return c.UploadGroupFile(ctx, accessToken, groupOpenID, filepath.Base(filePath), "", base64.StdEncoding.EncodeToString(data))
}

func (c *Client) doJSON(ctx context.Context, method, url, accessToken string, reqBody any, respBody any) error {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "QQBot "+accessToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("qq api %s %s failed: status %d trace %s body %s", method, url, resp.StatusCode, resp.Header.Get("X-Tps-trace-ID"), strings.TrimSpace(string(data)))
	}
	if respBody == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(respBody)
}
