//go:build linux

package wallpaper

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveWallpaperPathDecodesFileURI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gnome wallpaper.png")
	if err := writeTestPNG(path, color.NRGBA{R: 24, G: 48, B: 96, A: 255}); err != nil {
		t.Fatal(err)
	}

	resolved := resolveWallpaperPath("'file://"+strings.ReplaceAll(path, " ", "%20")+"'", true)
	if resolved != path {
		t.Fatalf("resolved path = %q, want %q", resolved, path)
	}
}

func TestResolveWallpaperPathAcceptsGNOMEJXLWallpaper(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gnome wallpaper.jxl")
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	resolved := resolveWallpaperPath("'file://"+strings.ReplaceAll(path, " ", "%20")+"'", true)
	if resolved != path {
		t.Fatalf("resolved path = %q, want %q", resolved, path)
	}
}

func TestResolveWallpaperXMLPathPrefersExistingImage(t *testing.T) {
	dir := t.TempDir()
	darkPath := filepath.Join(dir, "dark.png")
	lightPath := filepath.Join(dir, "light.png")
	xmlPath := filepath.Join(dir, "wallpaper.xml")

	if err := writeTestPNG(lightPath, color.NRGBA{R: 180, G: 120, B: 90, A: 255}); err != nil {
		t.Fatal(err)
	}
	if err := writeTestPNG(darkPath, color.NRGBA{R: 30, G: 40, B: 50, A: 255}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xmlPath, []byte(`<?xml version="1.0" encoding="UTF-8"?>
<wallpaper>
  <filename>`+lightPath+`</filename>
  <filename-dark>`+darkPath+`</filename-dark>
</wallpaper>`), 0644); err != nil {
		t.Fatal(err)
	}

	if resolved := resolveWallpaperXMLPath(xmlPath, true); resolved != darkPath {
		t.Fatalf("dark wallpaper path = %q, want %q", resolved, darkPath)
	}
	if resolved := resolveWallpaperXMLPath(xmlPath, false); resolved != lightPath {
		t.Fatalf("light wallpaper path = %q, want %q", resolved, lightPath)
	}
}

func writeTestPNG(path string, c color.NRGBA) error {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}
