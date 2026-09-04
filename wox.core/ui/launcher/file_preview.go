package launcher

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"wox/common"
	woxui "wox/ui/runtime"
	"wox/util"
)

// Automatic file previews stay cheap: text layout and Office/PDF handlers are deferred
// above these sizes, matching the Flutter-era details-first gate. After an explicit load,
// text is still bounded so minified one-line files cannot stall DirectWrite wrapping.
const (
	autoPreviewTextBytes         = 64 * 1024
	maxFilePreviewDisplayBytes   = 16 * 1024
	maxFilePreviewDisplayLines   = 200
	maxFilePreviewLineRunes      = 240
	officePreviewManualLoadBytes = 5 * 1024 * 1024
	tooLargePreviewBytes         = 50 * 1024 * 1024
	filePreviewAutoLoadDelay     = 180 * time.Millisecond
)

type filePreviewContent struct {
	Kind               string
	Text               string
	Image              woxImage
	WebViewData        string
	NativeFilePath     string
	NativeFileAutoLoad bool
	Tags               []previewTag
	Path               string
	Size               int64
	Modified           time.Time
	TypeLabel          string
	Limited            bool
	DisplayLines       int
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
	return filePreviewContent{Kind: "loading"}
}

// prepareFilePreview inspects the selected file without starting obsolete I/O during rapid selection.
func (a *App) prepareFilePreview(path string) {
	path = strings.TrimSpace(path)
	if path == "" || strings.HasPrefix(strings.ToLower(path), "http://") || strings.HasPrefix(strings.ToLower(path), "https://") {
		a.cancelScheduledFilePreview()
		return
	}
	extension := strings.ToLower(filepath.Ext(path))
	if extension == ".png" || extension == ".jpg" || extension == ".jpeg" || extension == ".gif" {
		a.cancelScheduledFilePreview()
		return
	}
	if _, loaded := a.filePreviews[path]; loaded {
		if a.filePreviewPendingPath != path {
			a.cancelScheduledFilePreview()
		}
		return
	}
	if a.fileRequests[path] {
		return
	}
	if a.filePreviewManualPaths[path] {
		a.cancelScheduledFilePreview()
		a.startFilePreviewLoad(path, extension, true)
		return
	}
	a.scheduleFilePreviewLoad(path, extension)
}

// scheduleFilePreviewLoad waits for selection to settle, matching Flutter's delayed file preview.
func (a *App) scheduleFilePreviewLoad(path, extension string) {
	if path == "" || a.filePreviewPendingPath == path {
		return
	}
	a.cancelScheduledFilePreview()
	a.filePreviewPendingPath = path
	a.filePreviewLoadGeneration++
	generation := a.filePreviewLoadGeneration
	a.filePreviewTimer = time.AfterFunc(filePreviewAutoLoadDelay, func() {
		if err := a.runOnUI("start delayed file preview", func() {
			a.startScheduledFilePreview(path, extension, generation)
		}); err != nil {
			util.GetLogger().Error(a.lifecycleCtx, "dispatch delayed file preview: "+err.Error())
		}
	})
}

// startScheduledFilePreview starts a delayed read only when that selection is still current.
func (a *App) startScheduledFilePreview(path, extension string, generation uint64) {
	if a.destroyed.Load() || a.filePreviewLoadGeneration != generation || a.filePreviewPendingPath != path {
		return
	}
	a.filePreviewTimer = nil
	a.filePreviewPendingPath = ""
	_, preview, visible := a.selectedPreviewForLifecycle()
	preview = a.resolvePreview(preview)
	if !visible || preview.PreviewType != "file" || strings.TrimSpace(preview.PreviewData) != path {
		return
	}
	if _, loaded := a.filePreviews[path]; loaded || a.fileRequests[path] {
		return
	}
	a.startFilePreviewLoad(path, extension, false)
}

// startFilePreviewLoad keeps both metadata inspection and content reads off the UI thread.
func (a *App) startFilePreviewLoad(path, extension string, forceLoad bool) {
	if a.fileRequests[path] {
		return
	}
	a.fileRequests[path] = true
	util.Go(a.lifecycleCtx, "load file preview", func() {
		a.loadFilePreview(path, extension, forceLoad)
	})
}

// cancelScheduledFilePreview drops a pending automatic load when the user has already moved on.
func (a *App) cancelScheduledFilePreview() {
	if a.filePreviewTimer != nil {
		a.filePreviewTimer.Stop()
		a.filePreviewTimer = nil
	}
	if a.filePreviewPendingPath == "" {
		return
	}
	a.filePreviewLoadGeneration++
	a.filePreviewPendingPath = ""
}

// requestManualFilePreview loads a deferred preview after the user asks for the full contents.
func (a *App) requestManualFilePreview(path string) {
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	if a.filePreviewManualPaths == nil {
		a.filePreviewManualPaths = map[string]bool{}
	}
	a.filePreviewManualPaths[path] = true
	delete(a.filePreviews, path)
	delete(a.fileRequests, path)
	a.prepareFilePreview(path)
	if a.window != nil {
		_ = a.window.Invalidate()
	}
}

// loadFilePreview applies every inspection result through the same bounded cache.
func (a *App) loadFilePreview(path, extension string, forceLoad bool) {
	content := inspectPreviewFile(path, extension, forceLoad)
	if err := a.runOnUI("apply file preview", func() {
		if len(a.filePreviews) >= 128 {
			// File previews are immutable during one query session; reset keeps ownership obvious.
			a.filePreviews = map[string]filePreviewContent{}
			a.fileRequests = map[string]bool{path: true}
			if a.filePreviewManualPaths[path] {
				a.filePreviewManualPaths = map[string]bool{path: true}
			} else {
				a.filePreviewManualPaths = map[string]bool{}
			}
		}
		a.filePreviews[path] = content
		a.reconcileSelectedPreviewOnUI()
		if a.window != nil {
			_ = a.window.Invalidate()
		}
	}); err != nil {
		util.GetLogger().Error(a.lifecycleCtx, "dispatch file preview result: "+err.Error())
	}
}

// inspectPreviewFile reads metadata first and only opens content within the preview budget.
func inspectPreviewFile(path, extension string, forceLoad bool) filePreviewContent {
	info, err := os.Stat(path)
	if err != nil {
		return filePreviewContent{Kind: "error", Text: fmt.Sprintf("Unable to inspect file:\n%s\n\n%v", path, err)}
	}
	typeLabel, tags := previewFileTypeAndTags(extension, info.Size())
	base := filePreviewMetadata(path, typeLabel, info, tags)
	if info.IsDir() {
		base.Kind = "info"
		base.Text = fmt.Sprintf("Folder\n\n%s\n\nModified %s", path, info.ModTime().Format(time.RFC1123))
		return base
	}
	if !forceLoad {
		if deferred, ok := deferredFilePreview(base, extension, info.Size()); ok {
			return deferred
		}
	}
	if isPDFPreviewExtension(extension) {
		if isTooLargeFilePreview(extension, info.Size()) {
			base.Kind = "too_large"
			return base
		}
		webViewData, err := buildPDFPreviewData(path)
		if err != nil {
			base.Kind = "error"
			base.Text = fmt.Sprintf("Unable to build PDF preview:\n%s\n\n%v", path, err)
			return base
		}
		base.Kind = "webview"
		base.WebViewData = webViewData
		return base
	}
	if isVideoPreviewExtension(extension) {
		webViewData, err := buildVideoPreviewData(path)
		if err != nil {
			base.Kind = "error"
			base.Text = fmt.Sprintf("Unable to build video preview:\n%s\n\n%v", path, err)
			return base
		}
		base.Kind = "webview"
		base.WebViewData = webViewData
		return base
	}
	if isAudioPreviewExtension(extension) {
		webViewData, err := buildAudioPreviewData(path)
		if err != nil {
			base.Kind = "error"
			base.Text = fmt.Sprintf("Unable to build audio preview:\n%s\n\n%v", path, err)
			return base
		}
		base.Kind = "webview"
		base.WebViewData = webViewData
		return base
	}
	if isOfficePreviewExtension(extension) {
		if isTooLargeFilePreview(extension, info.Size()) {
			base.Kind = "too_large"
			return base
		}
		base.Kind = "native_file"
		base.NativeFilePath = path
		base.NativeFileAutoLoad = true
		return base
	}
	file, err := os.Open(path)
	if err != nil {
		base.Kind = "error"
		base.Text = fmt.Sprintf("Unable to open file:\n%s\n\n%v", path, err)
		return base
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxFilePreviewDisplayBytes+1))
	if err != nil {
		base.Kind = "error"
		base.Text = fmt.Sprintf("Unable to read file:\n%s\n\n%v", path, err)
		return base
	}
	truncated := len(data) > maxFilePreviewDisplayBytes
	if truncated {
		data = data[:maxFilePreviewDisplayBytes]
		// Drop only an incomplete final rune; malformed bytes elsewhere still fail validation.
		start := len(data) - 1
		for start > 0 && !utf8.RuneStart(data[start]) {
			start--
		}
		if !utf8.FullRune(data[start:]) {
			data = data[:start]
		}
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		base.Kind = "info"
		base.Text = fmt.Sprintf("%s preview is not available yet.\n\n%s", typeLabel, path)
		return base
	}
	if extension == ".md" || extension == ".markdown" {
		// Preserve Markdown syntax; visual wrapping belongs to its renderer.
		lines := strings.SplitAfterN(string(data), "\n", maxFilePreviewDisplayLines+1)
		base.Kind = "markdown"
		base.Text = strings.Join(lines[:min(len(lines), maxFilePreviewDisplayLines)], "")
		base.DisplayLines = min(len(lines), maxFilePreviewDisplayLines)
		base.Limited = truncated || len(lines) > maxFilePreviewDisplayLines
		return base
	}
	text, lines, limited := boundFilePreviewText(string(data))
	base.Kind = "text"
	base.Text = text
	base.DisplayLines = lines
	base.Limited = truncated || limited
	return base
}

// boundFilePreviewText hard-wraps minified lines before the preview measurer sees them.
func boundFilePreviewText(text string) (string, int, bool) {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	if text == "" {
		return "", 0, false
	}
	var builder strings.Builder
	builder.Grow(min(len(text), maxFilePreviewDisplayBytes))
	lines := 0
	limited := false
	for index, paragraph := range strings.Split(text, "\n") {
		remaining := []rune(paragraph)
		if len(remaining) == 0 {
			if lines >= maxFilePreviewDisplayLines {
				limited = true
				break
			}
			if lines > 0 || index > 0 {
				builder.WriteByte('\n')
			}
			lines++
			continue
		}
		for len(remaining) > 0 {
			if lines >= maxFilePreviewDisplayLines {
				limited = true
				break
			}
			chunk := remaining
			if len(chunk) > maxFilePreviewLineRunes {
				chunk = remaining[:maxFilePreviewLineRunes]
				remaining = remaining[maxFilePreviewLineRunes:]
			} else {
				remaining = nil
			}
			if lines > 0 {
				builder.WriteByte('\n')
			}
			builder.WriteString(string(chunk))
			lines++
		}
		if limited {
			break
		}
	}
	return builder.String(), lines, limited
}

// deferredFilePreview keeps expensive text layout and native handlers off the automatic selection path.
func deferredFilePreview(base filePreviewContent, extension string, size int64) (filePreviewContent, bool) {
	if isTooLargeFilePreview(extension, size) {
		base.Kind = "too_large"
		return base, true
	}
	if shouldDeferFilePreview(extension, size) {
		base.Kind = "large"
		return base, true
	}
	return filePreviewContent{}, false
}

func shouldDeferFilePreview(extension string, size int64) bool {
	if isVideoPreviewExtension(extension) || isAudioPreviewExtension(extension) {
		return false
	}
	if isOfficePreviewExtension(extension) || isPDFPreviewExtension(extension) {
		return size > officePreviewManualLoadBytes
	}
	return size > autoPreviewTextBytes
}

func isTooLargeFilePreview(extension string, size int64) bool {
	if !isOfficePreviewExtension(extension) && !isPDFPreviewExtension(extension) {
		return false
	}
	return size > tooLargePreviewBytes
}

func previewFileTypeAndTags(extension string, size int64) (string, []previewTag) {
	typeLabel := strings.TrimPrefix(strings.ToUpper(extension), ".")
	if typeLabel == "" {
		typeLabel = "FILE"
	}
	return typeLabel, []previewTag{{Label: typeLabel}, {Label: formatFileSize(size)}}
}

func filePreviewMetadata(path, typeLabel string, info os.FileInfo, tags []previewTag) filePreviewContent {
	return filePreviewContent{Path: path, Size: info.Size(), Modified: info.ModTime(), TypeLabel: typeLabel, Tags: tags}
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

func formatFileSizeMegabytes(size int64) string {
	return fmt.Sprintf("%.1f", float64(size)/(1024*1024))
}

// onFilePreviewKey loads a deferred file preview when the documented preview hotkey is pressed.
func (a *App) onFilePreviewKey(event woxui.KeyEvent) bool {
	if !hotkeyMatches(primaryHotkey("l"), event) {
		return false
	}
	_, preview, visible := a.selectedPreviewForLifecycle()
	preview = a.resolvePreview(preview)
	if !visible || preview.PreviewType != "file" {
		return false
	}
	content := a.filePreviewFor(preview.PreviewData)
	switch content.Kind {
	case "large":
		a.requestManualFilePreview(preview.PreviewData)
		return true
	case "too_large":
		return true
	case "native_file":
		if content.NativeFileAutoLoad || a.nativeFilePreviewManualPath == content.NativeFilePath {
			return false
		}
		a.requestManualNativeFilePreview(content.NativeFilePath)
		return true
	default:
		return false
	}
}
