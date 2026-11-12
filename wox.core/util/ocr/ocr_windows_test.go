package ocr

import (
	"context"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"golang.org/x/sys/windows"
)

func TestWindowsMediaOCRRecognizesText(t *testing.T) {
	parsed, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: 56, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		t.Fatal(err)
	}
	defer face.Close()

	bitmap := image.NewRGBA(image.Rect(0, 0, 500, 130))
	draw.Draw(bitmap, bitmap.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	drawer := font.Drawer{Dst: bitmap, Src: image.NewUniform(color.Black), Face: face, Dot: fixed.P(25, 85)}
	drawer.DrawString("Windows OCR")
	path := filepath.Join(t.TempDir(), "剪贴板 OCR.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(png.Encode(file, bitmap), file.Close()); err != nil {
		t.Fatal(err)
	}

	// An unsupported hint exercises the fallback to an installed recognizer.
	result, err := recognizeWindowsWithMediaOCR(context.Background(), Request{ImagePath: path, Languages: []string{"zz-ZZ"}})
	const noOCRLanguagePack windows.Errno = 0x80070032 // HRESULT_FROM_WIN32(ERROR_NOT_SUPPORTED)
	if errors.Is(err, noOCRLanguagePack) || errors.Is(err, windows.Errno(windows.REGDB_E_CLASSNOTREG)) {
		t.Skipf("Windows Media OCR is unavailable on this host: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	recognized := strings.ToLower(strings.Join(strings.Fields(result.Text), ""))
	if result.Engine != EngineWindowsMediaOCR || !strings.Contains(recognized, "windows") {
		t.Fatalf("OCR using %s returned %q", result.Engine, result.Text)
	}
}
