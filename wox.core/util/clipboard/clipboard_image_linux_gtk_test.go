//go:build linux

package clipboard

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestLinuxClipboardPNG checks ownership, native format consumers, and clipboard-manager persistence.
// Opt in under a Linux desktop or Xvfb because the test replaces the desktop clipboard.
func TestLinuxClipboardPNG(t *testing.T) {
	if os.Getenv("WOX_TEST_NATIVE_CLIPBOARD") != "1" {
		t.Skip("set WOX_TEST_NATIVE_CLIPBOARD=1 under a Linux display to verify native PNG publication")
	}
	source := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	source.SetNRGBA(1, 0, color.NRGBA{R: 80, G: 40, B: 20, A: 128})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	path := filepath.Join(directory, "image.png")
	if err := os.WriteFile(path, encoded.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	flags, err := exec.Command("pkg-config", "--cflags", "--libs", "gtk+-3.0").CombinedOutput()
	if err != nil {
		t.Fatalf("find GTK compiler flags: %v\n%s", err, flags)
	}
	binary := filepath.Join(directory, "clipboard")
	arguments := []string{"-std=c11", "-Wall", "-Wextra", "-Werror", "testdata/clipboard_image_linux_gtk.c", "clipboard_image_linux_gtk.c", "-o", binary}
	arguments = append(arguments, strings.Fields(string(flags))...)
	if output, err := exec.Command("cc", arguments...).CombinedOutput(); err != nil {
		t.Fatalf("build native clipboard test: %v\n%s", err, output)
	}
	output, err := exec.Command(binary, "publish", path).CombinedOutput()
	if err != nil {
		t.Fatalf("native clipboard publication: %v\n%s", err, output)
	}
	if strings.Contains(string(output), "persistence=1") {
		if output, err := exec.Command(binary, "read", path).CombinedOutput(); err != nil {
			t.Fatalf("clipboard after provider exit: %v\n%s", err, output)
		}
	} else {
		t.Log("no clipboard manager on this display; provider-exit persistence was not exercised")
	}
}
