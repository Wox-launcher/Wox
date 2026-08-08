package launcher

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"wox/common"
	"wox/util"
)

const maxPreviewFileBytes = 512 * 1024

type filePreviewContent struct {
	Kind           string
	Text           string
	Image          woxImage
	WebViewData    string
	NativeFilePath string
	Tags           []previewTag
}

// filePreviewFor returns cached file content without starting I/O from the frame builder.
func (a *App) filePreviewFor(path string) filePreviewContent {
	path = strings.TrimSpace(path)
	if path == "" {
		return filePreviewContent{Kind: "error", Text: "File preview path is empty"}
	}
	if strings.HasPrefix(strings.ToLower(path), "http://") || strings.HasPrefix(strings.ToLower(path), "https://") {
		return filePreviewContent{Kind: "info", Text: "Remote file previews require the platform web surface.\n\n" + path}
	}
	extension := strings.ToLower(filepath.Ext(path))
	if extension == ".png" || extension == ".jpg" || extension == ".jpeg" || extension == ".gif" {
		return filePreviewContent{Kind: "image", Image: woxImage{ImageType: "absolute", ImageData: path}, Tags: []previewTag{{Label: strings.TrimPrefix(strings.ToUpper(extension), ".")}}}
	}
	if content, ok := a.filePreviews[path]; ok {
		return content
	}
	return filePreviewContent{Kind: "info", Text: "Loading file preview…"}
}

// prepareFilePreview starts local file inspection once before the next render.
func (a *App) prepareFilePreview(path string) {
	path = strings.TrimSpace(path)
	if path == "" || strings.HasPrefix(strings.ToLower(path), "http://") || strings.HasPrefix(strings.ToLower(path), "https://") {
		return
	}
	extension := strings.ToLower(filepath.Ext(path))
	if extension == ".png" || extension == ".jpg" || extension == ".jpeg" || extension == ".gif" {
		return
	}
	_, loaded := a.filePreviews[path]
	requested := a.fileRequests[path]
	if !loaded && !requested {
		a.fileRequests[path] = true
	}
	if !loaded && !requested {
		util.Go(a.lifecycleCtx, "load file preview", func() {
			a.loadFilePreview(path, extension)
		})
	}
}

func (a *App) loadFilePreview(path, extension string) {
	content := inspectPreviewFile(path, extension)
	if err := a.runOnUI("apply file preview", func() {
		if len(a.filePreviews) >= 128 {
			// File previews are immutable during one query session; reset keeps ownership obvious.
			a.filePreviews = map[string]filePreviewContent{}
			a.fileRequests = map[string]bool{path: true}
		}
		a.filePreviews[path] = content
		a.reconcileSelectedPreviewOnUI()
		if a.window != nil {
			_ = a.window.Invalidate()
		}
	}); err != nil {
		log.Printf("dispatch file preview result: %v", err)
	}
}

func inspectPreviewFile(path, extension string) filePreviewContent {
	info, err := os.Stat(path)
	if err != nil {
		return filePreviewContent{Kind: "error", Text: fmt.Sprintf("Unable to inspect file:\n%s\n\n%v", path, err)}
	}
	typeLabel := strings.TrimPrefix(strings.ToUpper(extension), ".")
	if typeLabel == "" {
		typeLabel = "FILE"
	}
	tags := []previewTag{{Label: typeLabel}, {Label: formatFileSize(info.Size())}}
	if info.IsDir() {
		return filePreviewContent{Kind: "info", Text: fmt.Sprintf("Folder\n\n%s\n\nModified %s", path, info.ModTime().Format(time.RFC1123)), Tags: tags}
	}
	if isPDFPreviewExtension(extension) {
		webViewData, err := buildPDFPreviewData(path)
		if err != nil {
			return filePreviewContent{Kind: "error", Text: fmt.Sprintf("Unable to build PDF preview:\n%s\n\n%v", path, err), Tags: tags}
		}
		return filePreviewContent{Kind: "webview", WebViewData: webViewData, Tags: tags}
	}
	if isVideoPreviewExtension(extension) {
		webViewData, err := buildVideoPreviewData(path)
		if err != nil {
			return filePreviewContent{Kind: "error", Text: fmt.Sprintf("Unable to build video preview:\n%s\n\n%v", path, err), Tags: tags}
		}
		return filePreviewContent{Kind: "webview", WebViewData: webViewData, Tags: tags}
	}
	if isAudioPreviewExtension(extension) {
		webViewData, err := buildAudioPreviewData(path)
		if err != nil {
			return filePreviewContent{Kind: "error", Text: fmt.Sprintf("Unable to build audio preview:\n%s\n\n%v", path, err), Tags: tags}
		}
		return filePreviewContent{Kind: "webview", WebViewData: webViewData, Tags: tags}
	}
	if isOfficePreviewExtension(extension) {
		return filePreviewContent{Kind: "native_file", NativeFilePath: path, Tags: tags}
	}
	file, err := os.Open(path)
	if err != nil {
		return filePreviewContent{Kind: "error", Text: fmt.Sprintf("Unable to open file:\n%s\n\n%v", path, err), Tags: tags}
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPreviewFileBytes+1))
	if err != nil {
		return filePreviewContent{Kind: "error", Text: fmt.Sprintf("Unable to read file:\n%s\n\n%v", path, err), Tags: tags}
	}
	truncated := len(data) > maxPreviewFileBytes
	if truncated {
		data = data[:maxPreviewFileBytes]
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return filePreviewContent{Kind: "info", Text: fmt.Sprintf("%s preview is not available yet.\n\n%s", typeLabel, path), Tags: tags}
	}
	text := string(data)
	if truncated {
		text += "\n\n… file preview truncated at 512 KB"
	}
	kind := "text"
	if extension == ".md" || extension == ".markdown" {
		kind = "markdown"
	}
	return filePreviewContent{Kind: kind, Text: text, Tags: tags}
}

// isVideoPreviewExtension identifies the formats rendered by the Flutter file-preview contract.
func isVideoPreviewExtension(extension string) bool {
	switch strings.ToLower(extension) {
	case ".mp4", ".m4v", ".mov", ".webm":
		return true
	default:
		return false
	}
}

// isPDFPreviewExtension identifies PDF files rendered by the platform WebView.
func isPDFPreviewExtension(extension string) bool {
	return strings.EqualFold(extension, ".pdf")
}

// isAudioPreviewExtension identifies the audio formats supported by the Flutter preview contract.
func isAudioPreviewExtension(extension string) bool {
	switch strings.ToLower(extension) {
	case ".mp3", ".wav", ".m4a", ".aac", ".flac", ".ogg", ".opus":
		return true
	default:
		return false
	}
}

// isOfficePreviewExtension identifies the Office formats delegated to a Windows shell preview handler.
func isOfficePreviewExtension(extension string) bool {
	switch strings.ToLower(extension) {
	case ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx":
		return true
	default:
		return false
	}
}

func buildFilePreviewMediaSource(path string) string {
	encodedPath := base64.RawURLEncoding.EncodeToString([]byte(path))
	return fmt.Sprintf("http://127.0.0.1:%d/preview/file/media?path=%s", common.GetServerPort(), url.QueryEscape(encodedPath))
}

// buildPDFPreviewData delegates PDF rendering to the embedded browser's native PDF surface.
func buildPDFPreviewData(path string) (string, error) {
	data, err := json.Marshal(webViewPreviewData{URL: buildFilePreviewMediaSource(path), CacheDisabled: true})
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// buildVideoPreviewData creates a native WebView document that streams a local video through Wox's loopback server.
func buildVideoPreviewData(path string) (string, error) {
	source := buildFilePreviewMediaSource(path)
	videoHTML := fmt.Sprintf(`<!doctype html>
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
html,
body {
  margin: 0;
  width: 100%%;
  height: 100%%;
  overflow: hidden;
  background: transparent;
  color-scheme: light dark;
}
body {
  display: flex;
  align-items: center;
  justify-content: center;
}
video {
  display: block;
  width: 100%%;
  height: 100%%;
  object-fit: contain;
  background: transparent;
}
</style>
</head>
<body>
<video controls preload="metadata" playsinline src="%s"></video>
<script>
(() => {
  const video = document.querySelector('video');
  if (!video) {
    return;
  }

  const showPreviewFrame = () => {
    if (video.currentTime !== 0 || !Number.isFinite(video.duration) || video.duration <= 0) {
      return;
    }

    try {
      video.currentTime = Math.min(0.1, Math.max(video.duration - 0.01, 0));
      video.pause();
    } catch (_) {}
  };

  video.addEventListener('loadedmetadata', showPreviewFrame, { once: true });
  video.addEventListener('seeked', () => video.pause(), { once: true });
})();
</script>
</body>
</html>`, html.EscapeString(source))
	data, err := json.Marshal(webViewPreviewData{HTML: videoHTML, CacheDisabled: true})
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// buildAudioPreviewData creates the compact inline player used by the Flutter file-preview contract.
func buildAudioPreviewData(path string) (string, error) {
	source := buildFilePreviewMediaSource(path)
	audioHTML := fmt.Sprintf(`<!doctype html>
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
html, body {
  margin: 0;
  width: 100%%;
  height: 100%%;
  background: transparent;
  color-scheme: light dark;
}
body {
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}
audio {
  width: calc(100%% - 28px);
  max-width: 760px;
}
</style>
</head>
<body>
<audio controls preload="metadata" src="%s"></audio>
</body>
</html>`, html.EscapeString(source))
	data, err := json.Marshal(webViewPreviewData{HTML: audioHTML, CacheDisabled: true})
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func formatFileSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	if size < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(size)/1024)
	}
	if size < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(size)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(size)/(1024*1024*1024))
}
