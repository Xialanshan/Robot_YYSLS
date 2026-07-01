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
	PythonBin        string
	PaddleScriptPath string
	Timeout          time.Duration
	Enabled          bool
}

const (
	defaultQQAPIBase  = "https://api.sgroup.qq.com"
	defaultQQTokenURL = "https://bots.qq.com/app/getAppAccessToken"
	defaultOCRTimeout = 20
	defaultOCRPython  = "python3"
	defaultOCRScript  = "scripts/paddle_ocr.py"
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
		PythonBin:        envOrDefault("OCR_PYTHON_BIN", defaultOCRPython),
		PaddleScriptPath: envOrDefault("OCR_PADDLE_SCRIPT", defaultOCRScript),
		Timeout:          time.Duration(timeoutSeconds) * time.Second,
		Enabled:          provider != "",
	}
}

func validateOCRConfig(cfg OCRConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if cfg.Provider != "paddle" {
		return errors.New("OCR_PROVIDER must be paddle")
	}
	if strings.TrimSpace(cfg.PythonBin) == "" {
		return errors.New("OCR_PYTHON_BIN is required when OCR_PROVIDER=paddle")
	}
	if strings.TrimSpace(cfg.PaddleScriptPath) == "" {
		return errors.New("OCR_PADDLE_SCRIPT is required when OCR_PROVIDER=paddle")
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
