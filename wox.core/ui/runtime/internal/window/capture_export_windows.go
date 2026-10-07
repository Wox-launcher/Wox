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
	if err := png.Encode(file, source); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
