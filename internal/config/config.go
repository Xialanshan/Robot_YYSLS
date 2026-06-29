package config

import (
	"errors"
	"os"
)

type Config struct {
	QQ   QQConfig
	HTTP HTTPConfig
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

const (
	defaultQQAPIBase  = "https://api.sgroup.qq.com"
	defaultQQTokenURL = "https://bots.qq.com/app/getAppAccessToken"
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
	}
	if cfg.QQ.AppID == "" {
		return Config{}, errors.New("QQ_BOT_APP_ID is required")
	}
	if cfg.QQ.AppSecret == "" {
		return Config{}, errors.New("QQ_BOT_APP_SECRET is required")
	}
	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
