//go:build windows

package woxui

import (
	"image"
	"image/color"
	"testing"

	"github.com/lxn/win"
)

func TestWindowsHICONFromImageCreatesAlphaIcon(t *testing.T) {
	source := mustWindowIconImage(t, 48, color.RGBA{R: 245, G: 196, B: 81, A: 255})
	icon, err := windowsHICONFromImage(source, 32)
	if err != nil {
		t.Fatalf("windowsHICONFromImage() error = %v", err)
	}
	if icon == 0 {
		t.Fatal("windowsHICONFromImage() returned an empty HICON")
	}
	if !win.DestroyIcon(icon) {
		t.Fatal("DestroyIcon failed for the created window icon")
	}
}

func TestWindowsBigIconSizeKeepsPluginResolution(t *testing.T) {
	source := mustWindowIconImage(t, 256, color.RGBA{R: 245, G: 196, B: 81, A: 255})
	if size := windowsBigIconSize(source); size != 256 {
		t.Fatalf("windowsBigIconSize() = %d, want 256 so the taskbar can downscale a sharp notes glyph", size)
	}
}

func mustWindowIconImage(t *testing.T, size int, fill color.RGBA) *Image {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, size, size))
	for offset := 0; offset < len(source.Pix); offset += 4 {
		source.Pix[offset] = fill.R
		source.Pix[offset+1] = fill.G
		source.Pix[offset+2] = fill.B
		source.Pix[offset+3] = fill.A
	}
	decoded, err := NewImageFromPackedRGBA(source)
	if err != nil {
		t.Fatalf("NewImageFromPackedRGBA() error = %v", err)
	}
	return decoded
}
