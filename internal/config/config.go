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
	Provider         string
	TencentSecretID  string
	TencentSecretKey string
	TencentRegion    string
	Timeout          time.Duration
	Enabled          bool
}

const (
	defaultQQAPIBase  = "https://api.sgroup.qq.com"
	defaultQQTokenURL = "https://bots.qq.com/app/getAppAccessToken"
	defaultOCRRegion  = "ap-shanghai"
	defaultOCRTimeout = 20
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
		Provider:         provider,
		TencentSecretID:  os.Getenv("TENCENTCLOUD_SECRET_ID"),
		TencentSecretKey: os.Getenv("TENCENTCLOUD_SECRET_KEY"),
		TencentRegion:    envOrDefault("TENCENTCLOUD_REGION", defaultOCRRegion),
		Timeout:          time.Duration(timeoutSeconds) * time.Second,
		Enabled:          provider != "",
	}
}

func validateOCRConfig(cfg OCRConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Provider != "tencent" {
		return errors.New("OCR_PROVIDER must be tencent")
	}
	if cfg.TencentSecretID == "" {
		return errors.New("TENCENTCLOUD_SECRET_ID is required when OCR_PROVIDER=tencent")
	}
	if cfg.TencentSecretKey == "" {
		return errors.New("TENCENTCLOUD_SECRET_KEY is required when OCR_PROVIDER=tencent")
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
