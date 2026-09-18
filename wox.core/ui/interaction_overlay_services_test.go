package ui

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"wox/common"
	"wox/util"
)

func TestFetchWebsiteIconLoadsDirectImage(t *testing.T) {
	pngBytes := testServicePNGBytes(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	t.Cleanup(server.Close)

	icon, err := NewCoreServices().FetchWebsiteIcon(context.Background(), "test", server.URL+"/icon.png")
	if err != nil {
		t.Fatalf("FetchWebsiteIcon() error = %v", err)
	}
	if icon.ImageType != common.WoxImageTypeBase64 || icon.ImageData == "" {
		t.Fatalf("icon = %+v, want an embedded image", icon)
	}
}

func TestFetchWebsiteIconReturnsImageHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	t.Cleanup(server.Close)

	_, err := NewCoreServices().FetchWebsiteIcon(context.Background(), "test", server.URL+"/icon.png")
	var status *util.HTTPStatusError
	if !errors.As(err, &status) || status.StatusCode != http.StatusNotFound {
		t.Fatalf("FetchWebsiteIcon() error = %v, want HTTP 404", err)
	}
}

func TestFetchWebsiteIconRejectsInvalidURL(t *testing.T) {
	_, err := NewCoreServices().FetchWebsiteIcon(context.Background(), "test", "javascript:alert(1)")
	if err == nil {
		t.Fatal("FetchWebsiteIcon() error = nil, want invalid URL")
	}
}

func testServicePNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}
