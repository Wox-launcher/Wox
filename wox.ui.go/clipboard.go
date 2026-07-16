package woxui

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
)

type clipboardImage struct {
	width  int
	height int
	stride int
	pixels []byte
	png    []byte
}

// WriteClipboardText publishes UTF-8 text through the native desktop clipboard.
func (w *Window) WriteClipboardText(text string) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.writeClipboardText(text)
}

// WriteClipboardImageFile decodes a raster image and publishes it through the native clipboard.
func (w *Window) WriteClipboardImageFile(filePath string) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	if filePath == "" {
		return errors.New("clipboard image file path is empty")
	}
	image, err := loadClipboardImage(filePath)
	if err != nil {
		return err
	}
	return w.native.writeClipboardImage(image)
}

// loadClipboardImage normalizes native clipboard input to straight-alpha RGBA and valid PNG bytes.
func loadClipboardImage(filePath string) (*clipboardImage, error) {
	encoded, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read clipboard image: %w", err)
	}
	source, format, err := image.Decode(bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("decode clipboard image: %w", err)
	}
	bounds := source.Bounds()
	if bounds.Empty() || bounds.Dx() > 16384 || bounds.Dy() > 16384 {
		return nil, fmt.Errorf("clipboard image dimensions are invalid: %dx%d", bounds.Dx(), bounds.Dy())
	}
	normalized := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(normalized, normalized.Bounds(), source, bounds.Min, draw.Src)
	if format != "png" {
		var buffer bytes.Buffer
		if err := png.Encode(&buffer, normalized); err != nil {
			return nil, fmt.Errorf("encode clipboard PNG: %w", err)
		}
		encoded = buffer.Bytes()
	}
	return &clipboardImage{width: bounds.Dx(), height: bounds.Dy(), stride: normalized.Stride, pixels: normalized.Pix, png: encoded}, nil
}
