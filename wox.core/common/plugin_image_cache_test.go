package common

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"wox/util"
	"wox/util/imagecache"
)

func TestPluginImageCacheIsolationAndReload(t *testing.T) {
	initConvertIconTestLocation(t)
	ctx := context.Background()
	pngPath := writeTestImage(t, 64, 64)
	pngBytes, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatal(err)
	}
	// A root-level SVG and a GetCacheFolder source must work without asset scanning.
	pluginFiles, err := util.GetLocation().EnsurePluginCacheDirectory("first")
	if err != nil {
		t.Fatal(err)
	}
	svgPath := filepath.Join(pluginFiles, "status.svg")
	if err := os.WriteFile(svgPath, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><path fill="red" d="M0 0h16v16H0z"/></svg>`), 0644); err != nil {
		t.Fatal(err)
	}
	sources := []WoxImage{
		NewWoxImageAbsolutePath(pngPath),
		NewWoxImageBase64("data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)),
		NewWoxImageAbsolutePath(svgPath),
		NewWoxImageAbsolutePath(writeTestGIF(t)),
	}
	sharedDirectory := t.TempDir()
	for _, source := range sources {
		t.Run(source.ImageType+filepath.Ext(source.ImageData), func(t *testing.T) {
			hash := source.Hash()
			first, err := ConvertPluginIcon(ctx, source, "first", sharedDirectory, IconConversion{})
			if err != nil {
				t.Fatal(err)
			}
			second, err := ConvertPluginIcon(ctx, source, "second", sharedDirectory, IconConversion{})
			if err != nil {
				t.Fatal(err)
			}
			if first.ImageData == second.ImageData {
				t.Fatal("plugins must not share artifacts")
			}
			if err := ClearPluginImageCache("first"); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(first.ImageData); !os.IsNotExist(err) {
				t.Fatalf("old artifact survived: %v", err)
			}
			if imagecache.IsKnownExistingDerivedPath(first.ImageData) {
				t.Fatal("positive path cache survived deletion")
			}
			if _, err := os.Stat(second.ImageData); err != nil {
				t.Fatalf("sibling artifact lost: %v", err)
			}
			if _, err := os.Stat(svgPath); err != nil {
				t.Fatalf("GetCacheFolder source lost: %v", err)
			}
			fresh, err := ConvertPluginIcon(ctx, source, "first", sharedDirectory, IconConversion{})
			if err != nil {
				t.Fatal(err)
			}
			if fresh.ImageData == first.ImageData {
				t.Fatal("UI would reuse the old image key")
			}
			if _, err := os.Stat(fresh.ImageData); err != nil {
				t.Fatal(err)
			}
			if source.Hash() != hash {
				t.Fatal("cache lifecycle changed image identity")
			}
		})
	}
	updated := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><path fill="blue" d="M0 0h16v16H0z"/></svg>`)
	if err := os.WriteFile(svgPath, updated, 0644); err != nil {
		t.Fatal(err)
	}
	if err := ClearPluginImageCache("first"); err != nil {
		t.Fatal(err)
	}
	fresh, err := ConvertPluginIcon(ctx, NewWoxImageAbsolutePath(svgPath), "first", "", IconConversion{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(fresh.ImageData)
	if err != nil || string(got) != string(updated) {
		t.Fatalf("SVG bytes were not refreshed: %s, %v", got, err)
	}
}

func TestPluginImageCacheRemoteAndStaleLazyRequest(t *testing.T) {
	initConvertIconTestLocation(t)
	ctx := context.Background()
	data, err := os.ReadFile(writeTestImage(t, 64, 64))
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write(data)
	}))
	defer server.Close()
	source := NewWoxImageUrl(server.URL + "/icon.png")
	config := IconConversion{AllowLazy: true}
	lazy, err := ConvertPluginIcon(ctx, source, "remote", "", config)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := ParseWoxLazyLoadImagePayload(lazy)
	if err != nil || payload.Source == nil {
		t.Fatalf("lazy payload: %v", err)
	}
	if requests.Load() != 0 {
		t.Fatal("lazy conversion fetched the URL eagerly")
	}
	candidate := NewWoxImageLazyLoadCandidate(source, ResultListIconSize)
	owned, err := ConvertPluginIcon(ctx, candidate, "remote", "", config)
	if err != nil {
		t.Fatal(err)
	}
	ownedPayload, err := ParseWoxLazyLoadImagePayload(owned)
	if err != nil || ownedPayload.CacheScope != payload.CacheScope {
		t.Fatalf("existing lazy candidate lost ownership: %+v, %v", ownedPayload, err)
	}

	config.AllowLazy = false
	warm, err := ConvertPluginIcon(ctx, source, "remote", "", config)
	if err != nil || warm.ImageType != WoxImageTypeAbsolutePath {
		t.Fatalf("warm: %+v", warm)
	}
	if err := ClearPluginImageCache("remote"); err != nil {
		t.Fatal(err)
	}
	config.CacheScope = payload.CacheScope
	if _, err := ConvertPluginIcon(ctx, source, "remote", "", config); err == nil {
		t.Fatal("stale lazy request was accepted")
	}
	if requests.Load() != 1 {
		t.Fatal("stale lazy request downloaded an image")
	}
	// Simulate restart: memory generations disappear, but removed disk files cannot return.
	root, _, err := pluginImageCacheFor("remote")
	if err != nil {
		t.Fatal(err)
	}
	pluginImageCaches.Delete(root)
	ClearConvertIconPathExistenceCache()
	config.CacheScope = ""
	fresh, err := ConvertPluginIcon(ctx, source, "remote", "", config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fresh.ImageData); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 {
		t.Fatal("restart reused a stale remote artifact")
	}
}

func TestPluginImageCacheRejectsUnsafeOwner(t *testing.T) {
	initConvertIconTestLocation(t)
	err := ClearPluginImageCache("../other")
	if err == nil {
		t.Fatal("unsafe plugin ID accepted")
	}
}
