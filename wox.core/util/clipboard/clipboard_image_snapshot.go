package clipboard

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"

	_ "golang.org/x/image/tiff"
)

var ErrImageTooManyPixels = errors.New("clipboard image exceeds pixel limit")

// ImageSnapshot owns the captured payload so decoding never rereads the live clipboard.
type ImageSnapshot struct {
	bounds image.Rectangle
	decode func() (image.Image, error)
}

func (s *ImageSnapshot) Bounds() image.Rectangle { return s.bounds }

// Decode checks dimensions before allocating pixels. Zero preserves unrestricted legacy reads.
func (s *ImageSnapshot) Decode(maxPixels int64) (image.Image, error) {
	w, h := int64(s.bounds.Dx()), int64(s.bounds.Dy())
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("decode image: invalid dimensions")
	}
	if maxPixels > 0 && w > maxPixels/h {
		return nil, ErrImageTooManyPixels
	}
	return s.decode()
}

// ReadImageSnapshot captures image bytes and dimensions without decoding the pixels.
func ReadImageSnapshot() (*ImageSnapshot, error) {
	snapshot, err := readImageSnapshot()
	if errors.Is(err, noDataErr) {
		return nil, nil
	}
	return snapshot, err
}

// encodedImageSnapshot keeps immutable encoded bytes until the worker decodes them.
func encodedImageSnapshot(data []byte) (*ImageSnapshot, error) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	return &ImageSnapshot{
		bounds: image.Rect(0, 0, config.Width, config.Height),
		decode: func() (image.Image, error) {
			img, _, err := image.Decode(bytes.NewReader(data))
			return img, err
		},
	}, nil
}
