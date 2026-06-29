package qqapi

import "context"

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

func (s APIReplySender) SendGroupReply(ctx context.Context, groupOpenID, content, eventID, msgID string, msgSeq int) error {
	token, err := s.Tokens.Token(ctx)
	if err != nil {
		return err
	}
	_, err = s.Client.SendGroupText(ctx, token, groupOpenID, content, eventID, msgID, msgSeq)
	return err
}

func (s APIReplySender) SendGroupTemplateFile(ctx context.Context, groupOpenID, templatePath, eventID, msgID string, msgSeq int) error {
	token, err := s.Tokens.Token(ctx)
	if err != nil {
		return err
	}
	upload, err := s.Client.UploadGroupLocalFile(ctx, token, groupOpenID, templatePath)
	if err != nil {
		return err
	}
	_, err = s.Client.SendGroupMedia(ctx, token, groupOpenID, upload.FileInfo, eventID, msgID, msgSeq)
	return err
}
