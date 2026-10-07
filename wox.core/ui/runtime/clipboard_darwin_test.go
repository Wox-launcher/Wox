//go:build darwin

package woxui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestDarwinClipboardPNG verifies native PNG/TIFF consumers and promises that survive normal provider exit.
// Opt in because the integration test changes the desktop clipboard.
func TestDarwinClipboardPNG(t *testing.T) {
	if os.Getenv("WOX_TEST_NATIVE_CLIPBOARD") != "1" {
		t.Skip("set WOX_TEST_NATIVE_CLIPBOARD=1 to verify native PNG and TIFF publication")
	}
	directory := t.TempDir()
	path := filepath.Join(directory, "image.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	source := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	source.SetNRGBA(1, 0, color.NRGBA{R: 80, G: 40, B: 20, A: 128})
	err = png.Encode(file, source)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("encode clipboard fixture: %v, %v", err, closeErr)
	}
	binary := filepath.Join(directory, "clipboard")
	command := exec.Command("clang", "-fblocks", "-Wno-deprecated-declarations", "-framework", "Cocoa", "-framework", "ImageIO",
		"testdata/clipboard_darwin.m", "clipboard_darwin.m", "-o", binary)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build native clipboard test: %v\n%s", err, output)
	}
	for _, mode := range []string{"publish", "read"} {
		if output, err := exec.Command(binary, mode, path).CombinedOutput(); err != nil {
			t.Fatalf("native clipboard %s: %v\n%s", mode, err, output)
		}
	}
}
