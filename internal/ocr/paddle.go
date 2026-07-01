package ocr

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type PaddleConfig struct {
	PythonBin  string
	ScriptPath string
	Timeout    time.Duration
}

type PaddleProvider struct {
	cfg    PaddleConfig
	runner commandRunner
}

type commandRunner func(ctx context.Context, pythonBin, scriptPath, imagePath string) ([]byte, error)

func NewPaddleProvider(cfg PaddleConfig) *PaddleProvider {
	if strings.TrimSpace(cfg.PythonBin) == "" {
		cfg.PythonBin = "python3"
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 20 * time.Second
	}
	return &PaddleProvider{
		cfg:    cfg,
		runner: runPaddleCommand,
	}
}

func (p *PaddleProvider) Recognize(ctx context.Context, image ImageInput) (Result, error) {
	if strings.TrimSpace(p.cfg.ScriptPath) == "" {
		return Result{}, fmt.Errorf("paddle OCR script path is required")
	}
	ctx, cancel := context.WithTimeout(ctx, p.cfg.Timeout)
	defer cancel()

	imagePath, cleanup, err := p.prepareImage(image)
	if err != nil {
		return Result{}, err
	}
	defer cleanup()

	output, err := p.runner(ctx, p.cfg.PythonBin, p.cfg.ScriptPath, imagePath)
	if err != nil {
		return Result{}, err
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

func (p *PaddleProvider) prepareImage(image ImageInput) (string, func(), error) {
	if image.Path != "" {
		return image.Path, func() {}, nil
	}
	if len(image.Data) == 0 {
		return "", nil, fmt.Errorf("image data is required")
	}
	tempDir, err := os.MkdirTemp("", "robot_yysls_paddle_*")
	if err != nil {
		return "", nil, err
	}
	imagePath := filepath.Join(tempDir, "image.png")
	if err := os.WriteFile(imagePath, image.Data, 0o600); err != nil {
		_ = os.RemoveAll(tempDir)
		return "", nil, err
	}
	return imagePath, func() { _ = os.RemoveAll(tempDir) }, nil
}

func runPaddleCommand(ctx context.Context, pythonBin, scriptPath, imagePath string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, pythonBin, scriptPath, imagePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			return nil, fmt.Errorf("run paddle OCR failed: %w", err)
		}
		return nil, fmt.Errorf("run paddle OCR failed: %w: %s", err, trimmed)
	}
	return output, nil
}
