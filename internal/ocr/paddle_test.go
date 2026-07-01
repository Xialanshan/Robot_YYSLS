package ocr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPaddleProviderRecognize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		_, _ = w.Write([]byte(`{"provider":"paddle","request_id":"req-id","items":[{"text":"会心率","confidence":0.99,"polygon":[{"x":1,"y":2},{"x":3,"y":2},{"x":3,"y":4},{"x":1,"y":4}]}]}`))
	}))
	defer server.Close()

	provider := NewPaddleProvider(PaddleConfig{BaseURL: server.URL, Timeout: 2 * time.Second})

	got, err := provider.Recognize(context.Background(), ImageInput{Data: []byte("image")})
	if err != nil {
		t.Fatalf("Recognize() error = %v", err)
	}
	if got.Provider != "paddle" || got.RequestID != "req-id" || len(got.Items) != 1 {
		t.Fatalf("result = %+v", got)
	}
	if got.Items[0].Text != "会心率" || len(got.Items[0].Polygon) != 4 {
		t.Fatalf("item = %+v", got.Items[0])
	}
}

func TestPaddleProviderReturnsHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "booting", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	provider := NewPaddleProvider(PaddleConfig{BaseURL: server.URL, Timeout: 2 * time.Second})
	if _, err := provider.Recognize(context.Background(), ImageInput{Data: []byte("image")}); err == nil {
		t.Fatal("Recognize() error = nil, want HTTP error")
	}
}
