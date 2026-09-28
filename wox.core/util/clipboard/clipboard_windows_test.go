package clipboard

import (
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"testing"
)

// TestNativeWritesSurviveOwnerCleanup checks every native writer after its temporary HWND is destroyed.
func TestNativeWritesSurviveOwnerCleanup(t *testing.T) {
	previous, err := Read()
	if err == nil {
		t.Cleanup(func() {
			if err := Write(previous); err != nil {
				t.Errorf("restore clipboard: %v", err)
			}
		})
	}

	text := "Wox 🤖"
	if err := WriteText(text); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadText(); err != nil || got != text {
		t.Fatalf("text after owner cleanup = %q, %v", got, err)
	}

	path := filepath.Join(t.TempDir(), "image.gif")
	img := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.White})
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	err = gif.Encode(file, img, nil)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("create GIF: %v, %v", err, closeErr)
	}
	if err := Write(&FilePathData{FilePaths: []string{path}}); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadFilePaths(); err != nil || len(got) != 1 || got[0] != path {
		t.Fatalf("files after owner cleanup = %v, %v", got, err)
	}
	if err := WriteAnimatedGIF(path); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadFilePaths(); err != nil || len(got) != 1 || got[0] != path {
		t.Fatalf("GIF file after owner cleanup = %v, %v", got, err)
	}
	if err := Write(&ImageData{Image: img}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := ReadImageSnapshot()
	if err != nil || snapshot == nil || snapshot.Bounds() != img.Bounds() {
		t.Fatalf("image after owner cleanup = %v, %v", snapshot, err)
	}
	decoded, err := snapshot.Decode(0)
	if err != nil || color.RGBAModel.Convert(decoded.At(0, 0)) != color.RGBAModel.Convert(img.At(0, 0)) {
		t.Fatalf("image pixels after owner cleanup: %v", err)
	}
}
