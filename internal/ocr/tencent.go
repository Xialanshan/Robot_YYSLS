package ocr

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	tencentOCRAction  = "GeneralAccurateOCR"
	tencentOCRService = "ocr"
	tencentOCRVersion = "2018-11-19"
	defaultEndpoint   = "https://ocr.tencentcloudapi.com"
)

type TencentConfig struct {
	SecretID  string
	SecretKey string
	Region    string
	Endpoint  string
	Timeout   time.Duration
}

type TencentProvider struct {
	cfg        TencentConfig
	httpClient *http.Client
	now        func() time.Time
}

func NewTencentProvider(cfg TencentConfig) *TencentProvider {
	if cfg.Endpoint == "" {
		cfg.Endpoint = defaultEndpoint
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 20 * time.Second
	}
	return &TencentProvider{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.Timeout},
		now:        time.Now,
	}
}

func (p *TencentProvider) Recognize(ctx context.Context, image ImageInput) (Result, error) {
	if p.cfg.SecretID == "" || p.cfg.SecretKey == "" {
		return Result{}, fmt.Errorf("tencent OCR credentials are required")
	}
	data := image.Data
	if len(data) == 0 && image.Path != "" {
		var err error
		data, err = os.ReadFile(image.Path)
		if err != nil {
			return Result{}, err
		}
	}
	if len(data) == 0 {
		return Result{}, fmt.Errorf("image data is required")
	}

	payload, err := json.Marshal(tencentOCRRequest{
		ImageBase64: base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		return Result{}, err
	}
	var resp tencentOCRResponse
	if err := p.do(ctx, payload, &resp); err != nil {
		return Result{}, err
	}
	if resp.Response.Error.Code != "" {
		return Result{}, fmt.Errorf("tencent OCR failed: %s %s", resp.Response.Error.Code, resp.Response.Error.Message)
	}
	return normalizeTencentResponse(resp), nil
}

func (p *TencentProvider) do(ctx context.Context, payload []byte, out any) error {
	endpoint, err := url.Parse(p.cfg.Endpoint)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	now := p.now().UTC()
	timestamp := now.Unix()
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", endpoint.Host)
	req.Header.Set("X-TC-Action", tencentOCRAction)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-TC-Version", tencentOCRVersion)
	if p.cfg.Region != "" {
		req.Header.Set("X-TC-Region", p.cfg.Region)
	}
	req.Header.Set("Authorization", p.authorization(endpoint.Host, payload, now))

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("tencent OCR HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, out)
}

func (p *TencentProvider) authorization(host string, payload []byte, now time.Time) string {
	date := now.Format("2006-01-02")
	hashedPayload := sha256Hex(payload)
	canonicalHeaders := "content-type:application/json; charset=utf-8\nhost:" + host + "\n"
	canonicalRequest := strings.Join([]string{
		http.MethodPost,
		"/",
		"",
		canonicalHeaders,
		"content-type;host",
		hashedPayload,
	}, "\n")
	credentialScope := date + "/" + tencentOCRService + "/tc3_request"
	stringToSign := strings.Join([]string{
		"TC3-HMAC-SHA256",
		strconv.FormatInt(now.Unix(), 10),
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
	secretDate := hmacSHA256([]byte("TC3"+p.cfg.SecretKey), date)
	secretService := hmacSHA256(secretDate, tencentOCRService)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))
	return "TC3-HMAC-SHA256 Credential=" + p.cfg.SecretID + "/" + credentialScope + ", SignedHeaders=content-type;host, Signature=" + signature
}

func normalizeTencentResponse(resp tencentOCRResponse) Result {
	items := make([]TextItem, 0, len(resp.Response.TextDetections))
	for _, detection := range resp.Response.TextDetections {
		polygon := make([]Point, 0, len(detection.Polygon))
		for _, point := range detection.Polygon {
			polygon = append(polygon, Point{X: point.X, Y: point.Y})
		}
		items = append(items, TextItem{
			Text:       detection.DetectedText,
			Confidence: detection.Confidence,
			Polygon:    polygon,
		})
	}
	return Result{
		Provider:  "tencent",
		RequestID: resp.Response.RequestID,
		Items:     items,
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(data))
	return mac.Sum(nil)
}

type tencentOCRRequest struct {
	ImageBase64 string `json:"ImageBase64"`
}

type tencentOCRResponse struct {
	Response struct {
		TextDetections []struct {
			DetectedText string  `json:"DetectedText"`
			Confidence   float64 `json:"Confidence"`
			Polygon      []struct {
				X int `json:"X"`
				Y int `json:"Y"`
			} `json:"Polygon"`
		} `json:"TextDetections"`
		Error struct {
			Code    string `json:"Code"`
			Message string `json:"Message"`
		} `json:"Error"`
		RequestID string `json:"RequestId"`
	} `json:"Response"`
}
