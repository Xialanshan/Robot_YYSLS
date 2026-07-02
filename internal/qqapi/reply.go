package qqapi

import (
	"context"
	"log"
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
	startedAt := time.Now()

	token, err := s.Tokens.Token(outboundCtx)
	if err != nil {
		// #region debug-point
		log.Printf("debug_send_group_reply_token_failed group=%q msg=%q event=%q msg_seq=%d err=%v", groupOpenID, msgID, eventID, msgSeq, err)
		// #endregion debug-point
		return err
	}
	// #region debug-point
	log.Printf("debug_send_group_reply_start group=%q msg=%q event=%q msg_seq=%d content_preview=%q", groupOpenID, msgID, eventID, msgSeq, truncateForDebug(content, 120))
	// #endregion debug-point
	_, err = s.Client.SendGroupText(outboundCtx, token, groupOpenID, content, eventID, msgID, msgSeq)
	// #region debug-point
	log.Printf("debug_send_group_reply_done group=%q msg=%q event=%q msg_seq=%d duration_ms=%d err=%v", groupOpenID, msgID, eventID, msgSeq, time.Since(startedAt).Milliseconds(), err)
	// #endregion debug-point
	return err
}

func (s APIReplySender) SendGroupTemplateFile(_ context.Context, groupOpenID, templatePath, eventID, msgID string, msgSeq int) error {
	outboundCtx, cancel := outboundContext()
	defer cancel()
	startedAt := time.Now()

	token, err := s.Tokens.Token(outboundCtx)
	if err != nil {
		// #region debug-point
		log.Printf("debug_send_group_template_token_failed group=%q msg=%q event=%q msg_seq=%d path=%q err=%v", groupOpenID, msgID, eventID, msgSeq, templatePath, err)
		// #endregion debug-point
		return err
	}
	// #region debug-point
	log.Printf("debug_send_group_template_start group=%q msg=%q event=%q msg_seq=%d path=%q", groupOpenID, msgID, eventID, msgSeq, templatePath)
	// #endregion debug-point
	upload, err := s.Client.UploadGroupLocalFile(outboundCtx, token, groupOpenID, templatePath)
	if err != nil {
		// #region debug-point
		log.Printf("debug_send_group_template_upload_failed group=%q msg=%q event=%q msg_seq=%d path=%q duration_ms=%d err=%v", groupOpenID, msgID, eventID, msgSeq, templatePath, time.Since(startedAt).Milliseconds(), err)
		// #endregion debug-point
		return err
	}
	_, err = s.Client.SendGroupMedia(outboundCtx, token, groupOpenID, upload.FileInfo, eventID, msgID, msgSeq)
	// #region debug-point
	log.Printf("debug_send_group_template_done group=%q msg=%q event=%q msg_seq=%d path=%q duration_ms=%d err=%v", groupOpenID, msgID, eventID, msgSeq, templatePath, time.Since(startedAt).Milliseconds(), err)
	// #endregion debug-point
	return err
}

func outboundContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}
