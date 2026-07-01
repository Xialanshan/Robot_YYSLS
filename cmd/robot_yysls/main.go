package main

import (
	"log"
	"net/http"

	"robot_yysls/internal/config"
	"robot_yysls/internal/flow"
	"robot_yysls/internal/ocr"
	"robot_yysls/internal/qqapi"
	"robot_yysls/internal/session"
	"robot_yysls/internal/style"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	styles, err := style.MustLoadDefaultRegistry()
	if err != nil {
		log.Fatal(err)
	}

	client := qqapi.NewClient(cfg.QQ.AppID, cfg.QQ.AppSecret, cfg.QQ.APIBase, cfg.QQ.TokenURL)
	store := session.NewMemoryStore()
	coordinator := &flow.Coordinator{
		Store:  store,
		Styles: styles,
	}
	var ocrProvider ocr.Provider
	if cfg.OCR.Enabled {
		ocrProvider = ocr.NewPaddleProvider(ocr.PaddleConfig{
			BaseURL: cfg.OCR.PaddleURL,
			Timeout: cfg.OCR.Timeout,
		})
	}
	server := &qqapi.WebhookServer{
		BotSecret: cfg.QQ.AppSecret,
		Flow:      coordinator,
		OCR: &qqapi.OCRProcessor{
			Store:  store,
			Styles: styles,
			OCR:    ocrProvider,
		},
		Sender: qqapi.APIReplySender{
			Client: client,
			Tokens: qqapi.ClientTokenProvider{
				Client: client,
			},
		},
	}

	mux := http.NewServeMux()
	mux.Handle("/qq/webhook", server)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("Robot_YYSLS is running.\nQQ webhook path: /qq/webhook\nHealth check: /healthz\n"))
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})

	log.Printf("Robot_YYSLS listening on %s", cfg.HTTP.Addr)
	log.Fatal(http.ListenAndServe(cfg.HTTP.Addr, mux))
}
