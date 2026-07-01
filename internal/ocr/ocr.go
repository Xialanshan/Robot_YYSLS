package ocr

import "context"

type Provider interface {
	Recognize(ctx context.Context, image ImageInput) (Result, error)
}

type ImageInput struct {
	Path string
	Data []byte
}

type Result struct {
	Provider  string     `json:"provider"`
	RequestID string     `json:"request_id"`
	Items     []TextItem `json:"items"`
}

type TextItem struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Polygon    []Point `json:"polygon"`
}

type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type FakeProvider struct {
	Result Result
	Err    error
}

func (p FakeProvider) Recognize(context.Context, ImageInput) (Result, error) {
	if p.Err != nil {
		return Result{}, p.Err
	}
	return p.Result, nil
}
