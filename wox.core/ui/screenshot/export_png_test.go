package screenshot

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestScreenshotExportRetainsClipboardPNG verifies shared encoding for opaque and transparent captures.
func TestScreenshotExportRetainsClipboardPNG(t *testing.T) {
	source := image.NewRGBA(image.Rect(10, 20, 12, 21))
	source.SetRGBA(11, 20, color.RGBA{R: 40, G: 20, B: 10, A: 128})
	for _, opaque := range []bool{false, true} {
		if opaque {
			source.SetRGBA(10, 20, color.RGBA{A: 255})
			source.SetRGBA(11, 20, color.RGBA{R: 40, G: 20, B: 10, A: 255})
		}
		name := "transparent"
		if opaque {
			name = "opaque"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "capture.png")
			var retained bytes.Buffer
			if err := writeScreenshotImageWithPNG(path, source, &retained); err != nil {
				t.Fatal(err)
			}
			written, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(written, retained.Bytes()) {
				t.Fatalf("clipboard PNG differs from history export: %v", err)
			}
			decoded, err := png.Decode(bytes.NewReader(retained.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			for x := 0; x < 2; x++ {
				if color.NRGBAModel.Convert(decoded.At(x, 0)) != color.NRGBAModel.Convert(source.At(x+10, 20)) {
					t.Fatalf("PNG changed corner or edge alpha: %v", decoded.At(x, 0))
				}
			}
		})
	}
}

// TestScreenshotExportPublishesCompleteFile preserves an existing history image when encoding fails.
func TestScreenshotExportPublishesCompleteFile(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "capture.png")
	previous := []byte("previous screenshot")
	if err := os.WriteFile(path, previous, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeScreenshotImage(path, image.NewRGBA(image.Rectangle{})); err == nil {
		t.Fatal("empty export accepted")
	}
	written, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(written, previous) {
		t.Fatalf("failed export replaced the previous history image: %v", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary export retained: %v, %v", entries, err)
	}
	source := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if err := writeScreenshotImage(path, source); err != nil {
		t.Fatal(err)
	}
	written, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(written)); err != nil {
		t.Fatal(err)
	}
}
