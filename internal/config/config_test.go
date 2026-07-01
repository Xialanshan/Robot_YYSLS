package config

import (
	"testing"
	"time"
)

func TestLoadRequiresCredentials(t *testing.T) {
	t.Setenv("QQ_BOT_APP_ID", "")
	t.Setenv("QQ_BOT_APP_SECRET", "")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want credential error")
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("QQ_BOT_APP_ID", "1904844772")
	t.Setenv("QQ_BOT_APP_SECRET", "secret")
	t.Setenv("QQ_BOT_API_BASE", "https://example.test")
	t.Setenv("QQ_BOT_TOKEN_URL", "https://token.example.test")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.QQ.AppID != "1904844772" {
		t.Fatalf("AppID = %q", cfg.QQ.AppID)
	}
	if cfg.QQ.AppSecret != "secret" {
		t.Fatalf("AppSecret = %q", cfg.QQ.AppSecret)
	}
	if cfg.QQ.APIBase != "https://example.test" {
		t.Fatalf("APIBase = %q", cfg.QQ.APIBase)
	}
	if cfg.QQ.TokenURL != "https://token.example.test" {
		t.Fatalf("TokenURL = %q", cfg.QQ.TokenURL)
	}
	if cfg.HTTP.Addr != ":8080" {
		t.Fatalf("HTTP.Addr = %q", cfg.HTTP.Addr)
	}
	if cfg.OCR.Enabled {
		t.Fatalf("OCR should be disabled by default: %+v", cfg.OCR)
	}
}

func TestLoadReadsOCRConfig(t *testing.T) {
	t.Setenv("QQ_BOT_APP_ID", "1904844772")
	t.Setenv("QQ_BOT_APP_SECRET", "secret")
	t.Setenv("OCR_PROVIDER", "paddle")
	t.Setenv("OCR_PADDLE_URL", "http://127.0.0.1:18081/ocr")
	t.Setenv("OCR_TIMEOUT_SECONDS", "15")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !cfg.OCR.Enabled || cfg.OCR.Provider != "paddle" {
		t.Fatalf("OCR config = %+v", cfg.OCR)
	}
	if cfg.OCR.PaddleURL != "http://127.0.0.1:18081/ocr" {
		t.Fatalf("OCR config = %+v", cfg.OCR)
	}
	if cfg.OCR.Timeout != 15*time.Second {
		t.Fatalf("Timeout = %s", cfg.OCR.Timeout)
	}
}

func TestLoadRejectsUnknownOCRProvider(t *testing.T) {
	t.Setenv("QQ_BOT_APP_ID", "1904844772")
	t.Setenv("QQ_BOT_APP_SECRET", "secret")
	t.Setenv("OCR_PROVIDER", "unknown")

	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want OCR provider error")
	}
}
