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
	Provider  string
	RequestID string
	Items     []TextItem
}

type TextItem struct {
	Text       string
	Confidence float64
	Polygon    []Point
}

type Point struct {
	X int
	Y int
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
