package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	QQ   QQConfig
	HTTP HTTPConfig
	OCR  OCRConfig
}

type QQConfig struct {
	AppID     string
	AppSecret string
	APIBase   string
	TokenURL  string
}

type HTTPConfig struct {
	Addr string
}

type OCRConfig struct {
	Provider  string
	PaddleURL string
	Timeout   time.Duration
	Enabled   bool
}

const (
	defaultQQAPIBase  = "https://api.sgroup.qq.com"
	defaultQQTokenURL = "https://bots.qq.com/app/getAppAccessToken"
	defaultOCRTimeout = 20
	defaultPaddleURL  = "http://127.0.0.1:18081/ocr"
)

func Load() (Config, error) {
	cfg := Config{
		QQ: QQConfig{
			AppID:     os.Getenv("QQ_BOT_APP_ID"),
			AppSecret: os.Getenv("QQ_BOT_APP_SECRET"),
			APIBase:   envOrDefault("QQ_BOT_API_BASE", defaultQQAPIBase),
			TokenURL:  envOrDefault("QQ_BOT_TOKEN_URL", defaultQQTokenURL),
		},
		HTTP: HTTPConfig{
			Addr: envOrDefault("HTTP_ADDR", ":8080"),
		},
		OCR: loadOCRConfig(),
	}
	if cfg.QQ.AppID == "" {
		return Config{}, errors.New("QQ_BOT_APP_ID is required")
	}
	if cfg.QQ.AppSecret == "" {
		return Config{}, errors.New("QQ_BOT_APP_SECRET is required")
	}
	if err := validateOCRConfig(cfg.OCR); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func loadOCRConfig() OCRConfig {
	provider := strings.TrimSpace(os.Getenv("OCR_PROVIDER"))
	timeoutSeconds := defaultOCRTimeout
	if raw := strings.TrimSpace(os.Getenv("OCR_TIMEOUT_SECONDS")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			timeoutSeconds = parsed
		}
	}
	return OCRConfig{
		Provider:  provider,
		PaddleURL: envOrDefault("OCR_PADDLE_URL", defaultPaddleURL),
		Timeout:   time.Duration(timeoutSeconds) * time.Second,
		Enabled:   provider != "",
	}
}

func validateOCRConfig(cfg OCRConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Provider != "paddle" {
		return errors.New("OCR_PROVIDER must be paddle")
	}
	if strings.TrimSpace(cfg.PaddleURL) == "" {
		return errors.New("OCR_PADDLE_URL is required when OCR_PROVIDER=paddle")
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
