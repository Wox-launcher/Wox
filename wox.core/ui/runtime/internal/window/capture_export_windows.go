//go:build windows

package window

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
)

// writeWindowsCapturePNG persists the automation capture after its native window has rendered.
func writeWindowsCapturePNG(path string, source image.Image) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	// PNG has a direct RGBA path. Walking the native BGRX image through At
	// boxes every pixel and repeats the channel conversion during encoding.
	if packed, ok := source.(*PackedBGRA); ok {
		rgba := image.NewRGBA(image.Rect(0, 0, packed.Bounds().Dx(), packed.Bounds().Dy()))
		packed.WriteRGBA(rgba, packed.Bounds().Min)
		source = rgba
	}
	// Automation compares decoded pixels, so favor latency over artifact size.
	// Slow compression can block the UI thread through an entire caret phase.
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(file, source); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
