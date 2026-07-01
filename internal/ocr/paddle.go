package ocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type PaddleConfig struct {
	BaseURL string
	Timeout time.Duration
}

type PaddleProvider struct {
	cfg        PaddleConfig
	httpClient *http.Client
}

func NewPaddleProvider(cfg PaddleConfig) *PaddleProvider {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = "http://127.0.0.1:18081/ocr"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 20 * time.Second
	}
	return &PaddleProvider{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
	}
}

func (p *PaddleProvider) Recognize(ctx context.Context, image ImageInput) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()

	data, err := p.prepareImageData(image)
	if err != nil {
		return Result{}, err
	}
	reqBody, err := json.Marshal(paddleOCRRequest{
		ImageBase64: base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL, bytes.NewReader(reqBody))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("call paddle OCR service failed: %w", err)
	}
	defer resp.Body.Close()
	output, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return Result{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("paddle OCR service HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(output)))
	}
	var result Result
	if err := json.Unmarshal(output, &result); err != nil {
		return Result{}, fmt.Errorf("parse paddle OCR output failed: %w", err)
	}
	if result.Provider == "" {
		result.Provider = "paddle"
	}
	return result, nil
}

func (p *PaddleProvider) prepareImageData(image ImageInput) ([]byte, error) {
	if len(image.Data) > 0 {
		return image.Data, nil
	}
	if image.Path == "" {
		return nil, fmt.Errorf("image data is required")
	}
	return os.ReadFile(image.Path)
}

type paddleOCRRequest struct {
	ImageBase64 string `json:"image_base64"`
}
