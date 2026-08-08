package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wox/common"
)

func TestInspectPreviewFileUsesWebViewForMP4(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "clip.mp4")
	if err := os.WriteFile(filePath, []byte{0, 1, 2, 3}, 0o600); err != nil {
		t.Fatalf("failed to write test media: %v", err)
	}

	previousPort := common.GetServerPort()
	common.SetServerPort(45678)
	defer common.SetServerPort(previousPort)

	content := inspectPreviewFile(filePath, ".mp4")
	if content.Kind != "webview" {
		t.Fatalf("MP4 preview kind = %q, want webview", content.Kind)
	}

	var data webViewPreviewData
	if err := json.Unmarshal([]byte(content.WebViewData), &data); err != nil {
		t.Fatalf("failed to decode video WebView data: %v", err)
	}
	if !data.CacheDisabled {
		t.Fatal("video WebView cache should be disabled")
	}
	if !strings.Contains(data.HTML, "<video controls") {
		t.Fatal("video preview HTML does not contain controls")
	}
	if !strings.Contains(data.HTML, "127.0.0.1:45678/preview/file/media") {
		t.Fatal("video preview HTML does not use the loopback media endpoint")
	}
}

func TestInspectPreviewFileKeepsMissingMP4AsError(t *testing.T) {
	content := inspectPreviewFile(filepath.Join(t.TempDir(), "missing.mp4"), ".mp4")
	if content.Kind != "error" {
		t.Fatalf("missing MP4 preview kind = %q, want error", content.Kind)
	}
}

func TestInspectPreviewFileUsesWebViewForPDF(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "document.pdf")
	if err := os.WriteFile(filePath, []byte("%PDF-1.7"), 0o600); err != nil {
		t.Fatalf("failed to write test PDF: %v", err)
	}

	previousPort := common.GetServerPort()
	common.SetServerPort(45679)
	defer common.SetServerPort(previousPort)

	content := inspectPreviewFile(filePath, ".pdf")
	if content.Kind != "webview" {
		t.Fatalf("PDF preview kind = %q, want webview", content.Kind)
	}
	var data webViewPreviewData
	if err := json.Unmarshal([]byte(content.WebViewData), &data); err != nil {
		t.Fatalf("failed to decode PDF WebView data: %v", err)
	}
	if !data.CacheDisabled {
		t.Fatal("PDF WebView cache should be disabled")
	}
	if !strings.Contains(data.URL, "127.0.0.1:45679/preview/file/media") {
		t.Fatalf("PDF preview URL does not use the loopback media endpoint: %q", data.URL)
	}
}

func TestInspectPreviewFileUsesWebViewForAudio(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "track.mp3")
	if err := os.WriteFile(filePath, []byte{0, 1, 2, 3}, 0o600); err != nil {
		t.Fatalf("failed to write test audio: %v", err)
	}

	previousPort := common.GetServerPort()
	common.SetServerPort(45680)
	defer common.SetServerPort(previousPort)

	content := inspectPreviewFile(filePath, ".mp3")
	if content.Kind != "webview" {
		t.Fatalf("audio preview kind = %q, want webview", content.Kind)
	}
	var data webViewPreviewData
	if err := json.Unmarshal([]byte(content.WebViewData), &data); err != nil {
		t.Fatalf("failed to decode audio WebView data: %v", err)
	}
	if !data.CacheDisabled {
		t.Fatal("audio WebView cache should be disabled")
	}
	if !strings.Contains(data.HTML, "<audio controls") {
		t.Fatal("audio preview HTML does not contain controls")
	}
	if !strings.Contains(data.HTML, "127.0.0.1:45680/preview/file/media") {
		t.Fatal("audio preview HTML does not use the loopback media endpoint")
	}
}

func TestInspectPreviewFileUsesNativeHandlerForOffice(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "document.docx")
	if err := os.WriteFile(filePath, []byte("office"), 0o600); err != nil {
		t.Fatalf("failed to write test Office file: %v", err)
	}

	content := inspectPreviewFile(filePath, ".docx")
	if content.Kind != "native_file" {
		t.Fatalf("Office preview kind = %q, want native_file", content.Kind)
	}
	if content.NativeFilePath != filePath {
		t.Fatalf("native Office preview path = %q, want %q", content.NativeFilePath, filePath)
	}
}
