package qqapi

import (
	"context"
	"time"
)

type TokenProvider interface {
	Token(ctx context.Context) (string, error)
}

type StaticTokenProvider struct {
	AccessToken string
}

func (p StaticTokenProvider) Token(context.Context) (string, error) {
	return p.AccessToken, nil
}

type ClientTokenProvider struct {
	Client *Client
}

func (p ClientTokenProvider) Token(ctx context.Context) (string, error) {
	token, err := p.Client.GetAccessToken(ctx)
	if err != nil {
		return "", err
	}
	return token.Token, nil
}

type APIReplySender struct {
	Client *Client
	Tokens TokenProvider
}

func (s APIReplySender) SendGroupReply(_ context.Context, groupOpenID, content, eventID, msgID string, msgSeq int) error {
	outboundCtx, cancel := outboundContext()
	defer cancel()

	token, err := s.Tokens.Token(outboundCtx)
	if err != nil {
		return err
	}
	_, err = s.Client.SendGroupText(outboundCtx, token, groupOpenID, content, eventID, msgID, msgSeq)
	return err
}

func (s APIReplySender) SendGroupTemplateFile(_ context.Context, groupOpenID, templatePath, eventID, msgID string, msgSeq int) error {
	outboundCtx, cancel := outboundContext()
	defer cancel()

	token, err := s.Tokens.Token(outboundCtx)
	if err != nil {
		return err
	}
	upload, err := s.Client.UploadGroupLocalFile(outboundCtx, token, groupOpenID, templatePath)
	if err != nil {
		return err
	}
	_, err = s.Client.SendGroupMedia(outboundCtx, token, groupOpenID, upload.FileInfo, eventID, msgID, msgSeq)
	return err
}

func outboundContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}
