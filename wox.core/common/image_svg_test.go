package common

import (
	"encoding/base64"
	"image/color"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// TestWoxImageSVGThemeColors covers every SVG source used by local notification conversion.
func TestWoxImageSVGThemeColors(t *testing.T) {
	initConvertIconTestLocation(t)
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" color="#112233"><path fill="var(--wox-theme-icon-color)" d="M0 0h8v32H0z"/><path stroke="var(--wox-theme-icon-color)" stroke-width="8" d="M12 0v32"/><path fill="#4D6BFE" d="M16 0h8v32h-8z"/><path fill="currentColor" d="M24 0h8v32h-8z"/></svg>`
	filePath := filepath.Join(t.TempDir(), "icon.svg")
	if err := os.WriteFile(filePath, []byte(svg), 0600); err != nil {
		t.Fatal(err)
	}
	cachedURL := NewWoxImageUrl("https://example.invalid/icon")
	cachePath, err := cachedURL.urlImageCachePath(t.Context(), cachedURL.ImageData)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(replaceImageCacheExt(cachePath, ".svg"), []byte(svg), 0600); err != nil {
		t.Fatal(err)
	}
	sources := []struct {
		name  string
		image WoxImage
	}{
		{"inline", NewWoxImageSvg(svg)},
		{"file", NewWoxImageAbsolutePath(filePath)},
		{"base64", NewWoxImageBase64("data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg)))},
		{"cached URL", cachedURL},
	}
	for _, source := range sources {
		t.Run(source.name, func(t *testing.T) {
			for _, dark := range []bool{false, true} {
				img, err := source.image.ToImageWithOptions(t.Context(), WoxImageRenderOptions{LocalOnly: true, Dark: dark})
				if err != nil {
					t.Fatalf("dark=%v: %v", dark, err)
				}
				iconColor := color.RGBA{A: 255}
				if dark {
					iconColor = color.RGBA{R: 255, G: 255, B: 255, A: 255}
				}
				want := []color.RGBA{iconColor, iconColor, {R: 77, G: 107, B: 254, A: 255}, {R: 17, G: 34, B: 51, A: 255}}
				for index, expected := range want {
					if got := color.RGBAModel.Convert(img.At(index*8+4, 16)); got != expected {
						t.Errorf("dark=%v region=%d: pixel=%v, want %v", dark, index, got, expected)
					}
				}
			}
		})
	}
	if _, err := sources[0].image.ToImageWithoutRemoteFetch(); err != nil {
		t.Fatalf("legacy local conversion: %v", err)
	}
}

// TestWoxImageSVGFailureReturnsNil prevents a failed SVG from hiding a nil pointer in image.Image.
func TestWoxImageSVGFailureReturnsNil(t *testing.T) {
	for _, svg := range []string{
		`<svg><path fill="var(--missing-color)" d="M0 0h8v8H0z"/></svg>`,
		`<svg><path d="M invalid"/></svg>`,
		`<svg><path`,
	} {
		img, err := NewWoxImageSvg(svg).ToImageWithoutRemoteFetch()
		if err == nil || img != nil {
			t.Fatalf("failed SVG conversion = (%T, %v), want (nil, error)", img, err)
		}
	}
}

// TestWoxImageLocalOnlyCacheMiss keeps notification conversion independent of remote availability.
func TestWoxImageLocalOnlyCacheMiss(t *testing.T) {
	initConvertIconTestLocation(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
	defer server.Close()
	for _, source := range []WoxImage{NewWoxImageUrl(server.URL + "/missing.svg"), NewWoxImageEmoji("🧪")} {
		img, err := source.ToImageWithOptions(t.Context(), WoxImageRenderOptions{LocalOnly: true, Dark: true})
		if err == nil || img != nil {
			t.Fatalf("local-only cache miss = (%T, %v), want (nil, error)", img, err)
		}
		if !strings.Contains(err.Error(), "image cache miss") {
			t.Fatalf("expected a local cache miss, got %v", err)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("local-only conversion made %d remote requests", requests.Load())
	}
}
