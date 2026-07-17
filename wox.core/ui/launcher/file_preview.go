package launcher

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

const maxPreviewFileBytes = 512 * 1024

type filePreviewContent struct {
	Kind  string
	Text  string
	Image woxImage
	Tags  []previewTag
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
	a.mu.RLock()
	if content, ok := a.filePreviews[path]; ok {
		a.mu.RUnlock()
		return content
	}
	a.mu.RUnlock()
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
	a.mu.Lock()
	_, loaded := a.filePreviews[path]
	requested := a.fileRequests[path]
	if !loaded && !requested {
		a.fileRequests[path] = true
	}
	a.mu.Unlock()
	if !loaded && !requested {
		go a.loadFilePreview(path, extension)
	}
}

func (a *App) loadFilePreview(path, extension string) {
	content := inspectPreviewFile(path, extension)
	a.mu.Lock()
	if len(a.filePreviews) >= 128 {
		// ponytail: File previews are immutable during one query session; reset keeps ownership obvious.
		a.filePreviews = map[string]filePreviewContent{}
		a.fileRequests = map[string]bool{path: true}
	}
	a.filePreviews[path] = content
	a.mu.Unlock()
	if a.window != nil {
		_ = a.window.Invalidate()
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
