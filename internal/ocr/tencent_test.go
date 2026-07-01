package ocr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTencentProviderRecognize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-TC-Action") != tencentOCRAction {
			t.Fatalf("X-TC-Action = %q", r.Header.Get("X-TC-Action"))
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "TC3-HMAC-SHA256 Credential=sid/") {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		var req tencentOCRRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		if req.ImageBase64 == "" {
			t.Fatal("ImageBase64 is empty")
		}
		_, _ = w.Write([]byte(`{
			"Response": {
				"TextDetections": [{
					"DetectedText": "会心率",
					"Confidence": 99,
					"Polygon": [{"X":1,"Y":2},{"X":3,"Y":2},{"X":3,"Y":4},{"X":1,"Y":4}]
				}],
				"RequestId": "req-id"
			}
		}`))
	}))
	defer server.Close()

	provider := NewTencentProvider(TencentConfig{
		SecretID:  "sid",
		SecretKey: "skey",
		Region:    "ap-shanghai",
		Endpoint:  server.URL,
		Timeout:   time.Second,
	})
	provider.now = func() time.Time {
		return time.Unix(1700000000, 0)
	}

	got, err := provider.Recognize(context.Background(), ImageInput{Data: []byte("image")})
	if err != nil {
		t.Fatalf("Recognize() error = %v", err)
	}
	if got.Provider != "tencent" || got.RequestID != "req-id" || len(got.Items) != 1 {
		t.Fatalf("result = %+v", got)
	}
	if got.Items[0].Text != "会心率" || len(got.Items[0].Polygon) != 4 {
		t.Fatalf("item = %+v", got.Items[0])
	}
}

func TestTencentProviderReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"Response":{"Error":{"Code":"AuthFailure","Message":"bad secret"},"RequestId":"req-id"}}`))
	}))
	defer server.Close()

	provider := NewTencentProvider(TencentConfig{
		SecretID:  "sid",
		SecretKey: "skey",
		Endpoint:  server.URL,
	})

	if _, err := provider.Recognize(context.Background(), ImageInput{Data: []byte("image")}); err == nil {
		t.Fatal("Recognize() error = nil, want API error")
	}
}
