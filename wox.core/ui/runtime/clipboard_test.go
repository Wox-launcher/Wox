package woxui

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
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

// TestClipboardImagePublishesTransparentPNG preserves window corner alpha for apps that ignore DIB alpha.
func TestClipboardImagePublishesTransparentPNG(t *testing.T) {
	source := image.NewRGBA(image.Rect(20, 30, 22, 31))
	source.SetRGBA(21, 30, color.RGBA{R: 40, G: 20, B: 10, A: 128})
	clipboard, err := newClipboardImage(source, nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, format, err := image.Decode(bytes.NewReader(clipboard.png))
	if err != nil || format != "png" {
		t.Fatalf("transparent clipboard PNG: format=%q err=%v", format, err)
	}
	if _, _, _, alpha := decoded.At(0, 0).RGBA(); alpha != 0 {
		t.Fatal("clipboard flattened empty window corner")
	}
	if _, _, _, alpha := decoded.At(1, 0).RGBA(); alpha != 128*257 {
		t.Fatal("clipboard changed antialiased edge alpha")
	}
	opaque := image.NewRGBA(image.Rect(0, 0, 1, 1))
	opaque.SetRGBA(0, 0, color.RGBA{A: 255})
	clipboard, err = newClipboardImage(opaque, nil)
	if err != nil || len(clipboard.png) != 0 {
		t.Fatal("opaque capture unnecessarily encoded PNG")
	}
}

// TestClipboardImageReusesEncodedPNG checks that export bytes retain alpha without another compression pass.
func TestClipboardImageReusesEncodedPNG(t *testing.T) {
	source := image.NewRGBA(image.Rect(10, 20, 12, 21))
	source.SetRGBA(11, 20, color.RGBA{R: 40, G: 20, B: 10, A: 128})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	clipboard, err := newClipboardImage(source, encoded.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(clipboard.png) != encoded.Len() || &clipboard.png[0] != &encoded.Bytes()[0] {
		t.Fatal("clipboard did not reuse the exported PNG")
	}
	if clipboard.width != 2 || clipboard.height != 1 {
		t.Fatalf("clipboard dimensions = %dx%d", clipboard.width, clipboard.height)
	}
	if clipboardUsesEncodedPNG {
		if len(clipboard.pixels) != 0 {
			t.Fatal("native PNG publication retained an uncompressed raster")
		}
	} else if !bytes.Equal(clipboard.pixels, []byte{0, 0, 0, 0, 79, 39, 19, 128}) {
		t.Fatalf("clipboard changed straight-alpha pixels: %v", clipboard.pixels)
	}
}
