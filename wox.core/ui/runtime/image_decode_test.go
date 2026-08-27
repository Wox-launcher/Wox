package woxui

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestDecodeImageMaxDownscalesLargeRasters(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 80, 40))
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatalf("encode: %v", err)
	}
	full, err := DecodeImage(bytes.NewReader(encoded.Bytes()))
	if err != nil || full.Width != 80 || full.Height != 40 {
		t.Fatalf("full decode = %#v %v", full, err)
	}
	limited, err := DecodeImageMax(bytes.NewReader(encoded.Bytes()), 20)
	if err != nil {
		t.Fatalf("limited decode: %v", err)
	}
	if limited.Width != 20 || limited.Height != 10 {
		t.Fatalf("limited size = %dx%d, want 20x10", limited.Width, limited.Height)
	}
}
