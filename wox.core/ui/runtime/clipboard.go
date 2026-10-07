package woxui

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
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

// FlushClipboard materializes promised native formats before the process exits.
func FlushClipboard() error {
	return flushClipboard()
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

// WriteClipboardImage publishes in-memory pixels, adding PNG when native formats would lose alpha.
func (w *Window) WriteClipboardImage(source image.Image) error {
	return w.WriteClipboardImageWithPNG(source, nil)
}

// WriteClipboardImageWithPNG reuses a PNG encoded from the same pixels instead of compressing transparent images twice.
func (w *Window) WriteClipboardImageWithPNG(source image.Image, encodedPNG []byte) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	clipboard, err := newClipboardImage(source, encodedPNG)
	if err != nil {
		return err
	}
	return w.native.writeClipboardImage(clipboard)
}

// loadClipboardImage preserves PNG bytes when already available and otherwise publishes pixels only.
func loadClipboardImage(filePath string) (*clipboardImage, error) {
	encoded, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read clipboard image: %w", err)
	}
	source, format, err := image.Decode(bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("decode clipboard image: %w", err)
	}
	if format != "png" {
		encoded = nil
	}
	return newClipboardImage(source, encoded)
}

// newClipboardImage prepares encoded PNG or straight-alpha pixels for the platform clipboard.
func newClipboardImage(source image.Image, encodedPNG []byte) (*clipboardImage, error) {
	if source == nil {
		return nil, errors.New("clipboard image is empty")
	}
	bounds := source.Bounds()
	if bounds.Empty() || bounds.Dx() > 16384 || bounds.Dy() > 16384 {
		return nil, fmt.Errorf("clipboard image dimensions are invalid: %dx%d", bounds.Dx(), bounds.Dy())
	}
	if clipboardUsesEncodedPNG && len(encodedPNG) > 0 {
		// Encoded native publication needs no additional normalized full-size raster.
		return &clipboardImage{width: bounds.Dx(), height: bounds.Dy(), png: encodedPNG}, nil
	}
	normalized := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(normalized, normalized.Bounds(), source, bounds.Min, draw.Src)
	// Windows' compatibility DIB format is often read without alpha. Publish PNG too for transparent captures.
	if len(encodedPNG) == 0 && !normalized.Opaque() {
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, normalized); err != nil {
			return nil, fmt.Errorf("encode transparent clipboard image: %w", err)
		}
		encodedPNG = encoded.Bytes()
	}
	return &clipboardImage{width: bounds.Dx(), height: bounds.Dy(), stride: normalized.Stride, pixels: normalized.Pix, png: encodedPNG}, nil
}
