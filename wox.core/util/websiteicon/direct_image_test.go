package websiteicon

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"wox/common"
)

func TestImageRequestHeadersForGitHub(t *testing.T) {
	headers := imageRequestHeaders("https://private-user-images.githubusercontent.com/1/a.png?jwt=token")
	if headers["Referer"] != "https://github.com/" {
		t.Fatalf("Referer = %q, want https://github.com/", headers["Referer"])
	}
	if headers["Accept"] == "" {
		t.Fatal("Accept header is required")
	}
}

func TestIsDirectImageURL(t *testing.T) {
	if !IsDirectImageURL("https://private-user-images.githubusercontent.com/1/a.png?jwt=token") {
		t.Fatal("signed png URL should be treated as a direct image")
	}
	if IsDirectImageURL("https://github.com/Wox-launcher/Wox") {
		t.Fatal("website URL should not be treated as a direct image")
	}
}

func TestFetchDirectImageSendsBrowserImageHeaders(t *testing.T) {
	pngBytes := testPNGBytes(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "" || r.Header.Get("Referer") == "" {
			http.Error(w, "missing image headers", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	t.Cleanup(server.Close)

	if _, err := FetchDirectImage(context.Background(), server.URL+"/icon.png"); err != nil {
		t.Fatalf("FetchDirectImage() error = %v", err)
	}
}

func TestFetchDirectImage(t *testing.T) {
	pngBytes := testPNGBytes(t)
	svgBody := `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><rect width="16" height="16" fill="#00aa00"/></svg>`

	t.Run("png", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)
		}))
		t.Cleanup(server.Close)

		icon, err := FetchDirectImage(context.Background(), server.URL+"/icon.png")
		if err != nil {
			t.Fatalf("FetchDirectImage() error = %v", err)
		}
		if icon.ImageType != common.WoxImageTypeBase64 || icon.ImageData == "" {
			t.Fatalf("icon = %+v, want an embedded image", icon)
		}
	})

	t.Run("png without content type", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(pngBytes)
		}))
		t.Cleanup(server.Close)

		if _, err := FetchDirectImage(context.Background(), server.URL+"/icon"); err != nil {
			t.Fatalf("FetchDirectImage() error = %v", err)
		}
	})

	t.Run("svg", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/svg+xml")
			_, _ = w.Write([]byte(svgBody))
		}))
		t.Cleanup(server.Close)

		if _, err := FetchDirectImage(context.Background(), server.URL+"/icon.svg"); err != nil {
			t.Fatalf("FetchDirectImage() error = %v", err)
		}
	})

	t.Run("html", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<!DOCTYPE html><html><body>not an image</body></html>"))
		}))
		t.Cleanup(server.Close)

		_, err := FetchDirectImage(context.Background(), server.URL+"/")
		if !errors.Is(err, ErrNotAnImage) {
			t.Fatalf("FetchDirectImage() error = %v, want ErrNotAnImage", err)
		}
	})

	t.Run("too large", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)
			_, _ = w.Write(bytes.Repeat([]byte{0}, maxDirectImageBytes))
		}))
		t.Cleanup(server.Close)

		_, err := FetchDirectImage(context.Background(), server.URL+"/huge.png")
		if err == nil || !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("FetchDirectImage() error = %v, want a size error", err)
		}
	})
}

func testPNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}
