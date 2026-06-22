package ui

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandlePreviewFileMediaSupportsRangeRequests(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "clip.mp4")
	content := []byte{0, 1, 2, 3, 4, 5}
	if err := os.WriteFile(filePath, content, 0o600); err != nil {
		t.Fatalf("failed to write test media: %v", err)
	}

	encodedPath := base64.URLEncoding.EncodeToString([]byte(filePath))
	request := httptest.NewRequest(http.MethodGet, "/preview/file/media?path="+url.QueryEscape(encodedPath), nil)
	request.Header.Set("Range", "bytes=1-3")
	response := httptest.NewRecorder()

	handlePreviewFileMedia(response, request)

	if response.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want %d, body=%q", response.Code, http.StatusPartialContent, response.Body.String())
	}
	if got := response.Body.Bytes(); !bytes.Equal(got, content[1:4]) {
		t.Fatalf("body = %v, want %v", got, content[1:4])
	}
	if got := response.Header().Get("Accept-Ranges"); got != "bytes" {
		t.Fatalf("Accept-Ranges = %q, want bytes", got)
	}
	if got := response.Header().Get("Content-Range"); got != "bytes 1-3/6" {
		t.Fatalf("Content-Range = %q, want bytes 1-3/6", got)
	}
	if got := response.Header().Get("Content-Type"); !strings.HasPrefix(got, "video/mp4") {
		t.Fatalf("Content-Type = %q, want video/mp4", got)
	}
}

func TestPreviewFileMediaRouteRegistered(t *testing.T) {
	if routers["/preview/file/media"] == nil {
		t.Fatal("preview file media route is not registered")
	}
}

func TestResolvePreviewFileMediaContentType(t *testing.T) {
	tests := map[string]string{
		"clip.mp4":   "video/mp4",
		"clip.m4v":   "video/mp4",
		"clip.mov":   "video/quicktime",
		"clip.webm":  "video/webm",
		"track.mp3":  "audio/mpeg",
		"track.wav":  "audio/wav",
		"track.m4a":  "audio/mp4",
		"track.aac":  "audio/aac",
		"track.flac": "audio/flac",
		"track.ogg":  "audio/ogg",
		"track.opus": "audio/ogg",
	}

	for fileName, want := range tests {
		t.Run(fileName, func(t *testing.T) {
			if got := resolvePreviewFileMediaContentType(fileName); got != want {
				t.Fatalf("content type = %q, want %q", got, want)
			}
		})
	}
}
