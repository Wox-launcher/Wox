package launcher

import (
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDemoWallpaperCacheKeyUsesImageContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallpaper.png")
	if err := os.WriteFile(path, []byte("first wallpaper"), 0644); err != nil {
		t.Fatal(err)
	}
	first, err := demoWallpaperCacheKey(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal(err)
	}
	unchanged, err := demoWallpaperCacheKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged != first {
		t.Fatalf("cache key changed with mtime only: %q != %q", unchanged, first)
	}
	if err := os.WriteFile(path, []byte("second wallpaper"), 0644); err != nil {
		t.Fatal(err)
	}
	changed, err := demoWallpaperCacheKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if changed == first {
		t.Fatal("cache key did not change with wallpaper content")
	}
}

func TestWriteDemoWallpaperCachePublishesDecodableImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wallpaper.png")
	source := image.NewNRGBA(image.Rect(0, 0, 7, 5))
	if err := writeDemoWallpaperCache(path, source); err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeDemoWallpaperCacheFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Width != 7 || decoded.Height != 5 {
		t.Fatalf("cached image size = %dx%d, want 7x5", decoded.Width, decoded.Height)
	}
}

func TestDecodeDemoWallpaperCreatesAndReusesProcessedCache(t *testing.T) {
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "source.png")
	if err := writeDemoWallpaperCache(sourcePath, image.NewNRGBA(image.Rect(0, 0, 32, 24))); err != nil {
		t.Fatal(err)
	}
	cacheDirectory := filepath.Join(directory, "cache")
	wallpaper, blurred, err := decodeDemoWallpaperWithCache(sourcePath, true, cacheDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if wallpaper.Width != demoWallpaperWidth || wallpaper.Height != demoWallpaperHeight {
		t.Fatalf("wallpaper size = %dx%d", wallpaper.Width, wallpaper.Height)
	}
	if blurred.Width != demoWallpaperBlurredWidth || blurred.Height != demoWallpaperBlurredHeight {
		t.Fatalf("blurred size = %dx%d", blurred.Width, blurred.Height)
	}
	entries, err := os.ReadDir(cacheDirectory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("cache entries = %d, want normal and blurred previews", len(entries))
	}
	old := time.Now().Add(-time.Hour)
	for _, entry := range entries {
		if err := os.Chtimes(filepath.Join(cacheDirectory, entry.Name()), old, old); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := decodeDemoWallpaperWithCache(sourcePath, true, cacheDirectory); err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		info, err := os.Stat(filepath.Join(cacheDirectory, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if !info.ModTime().After(old) {
			t.Fatalf("cache %s was not touched after reuse", entry.Name())
		}
	}
}
