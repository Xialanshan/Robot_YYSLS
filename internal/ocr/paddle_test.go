package ocr

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPaddleProviderRecognize(t *testing.T) {
	provider := NewPaddleProvider(PaddleConfig{
		PythonBin:  "python3",
		ScriptPath: "scripts/paddle_ocr.py",
	})
	provider.runner = func(_ context.Context, pythonBin, scriptPath, imagePath string) ([]byte, error) {
		if pythonBin != "python3" || scriptPath != "scripts/paddle_ocr.py" {
			t.Fatalf("pythonBin/scriptPath = %q %q", pythonBin, scriptPath)
		}
		if !strings.HasSuffix(imagePath, ".png") {
			t.Fatalf("imagePath = %q", imagePath)
		}
		return []byte(`{"provider":"paddle","request_id":"req-id","items":[{"text":"会心率","confidence":0.99,"polygon":[{"x":1,"y":2},{"x":3,"y":2},{"x":3,"y":4},{"x":1,"y":4}]}]}`), nil
	}

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

func TestPaddleProviderUsesImagePathDirectly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	provider := NewPaddleProvider(PaddleConfig{
		PythonBin:  "python3",
		ScriptPath: "scripts/paddle_ocr.py",
	})
	provider.runner = func(_ context.Context, _, _, imagePath string) ([]byte, error) {
		if imagePath != path {
			t.Fatalf("imagePath = %q, want %q", imagePath, path)
		}
		return []byte(`{"items":[]}`), nil
	}

	if _, err := provider.Recognize(context.Background(), ImageInput{Path: path}); err != nil {
		t.Fatalf("Recognize() error = %v", err)
	}
}
