package woxui

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadClipboardImageDoesNotReencodeJPEG(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clipboard.jpg")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(file, image.NewRGBA(image.Rect(0, 0, 16, 12)), &jpeg.Options{Quality: 90}); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	clipboard, err := loadClipboardImage(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(clipboard.png) != 0 {
		t.Fatal("JPEG clipboard input was re-encoded as PNG")
	}
	if clipboard.width != 16 || clipboard.height != 12 || len(clipboard.pixels) == 0 {
		t.Fatalf("clipboard image = %dx%d with %d pixel bytes", clipboard.width, clipboard.height, len(clipboard.pixels))
	}
}
