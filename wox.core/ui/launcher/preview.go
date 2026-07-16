package launcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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

type previewListData struct {
	Items []previewListItem `json:"items"`
}

type previewListItem struct {
	Icon     *woxImage    `json:"icon"`
	Title    string       `json:"title"`
	Subtitle string       `json:"subtitle"`
	Tails    []resultTail `json:"tails"`
}

// resolvePreview replaces core's deferred preview reference without blocking the frame builder.
func (a *App) resolvePreview(preview queryPreview) queryPreview {
	if preview.PreviewType != "remote" {
		return normalizePreviewMetadata(preview)
	}
	key := strings.TrimSpace(preview.PreviewData)
	if key == "" {
		return queryPreview{PreviewType: "text", PreviewData: "Remote preview path is empty"}
	}
	a.mu.Lock()
	if resolved, ok := a.remotePreviews[key]; ok {
		a.mu.Unlock()
		return normalizePreviewMetadata(resolved)
	}
	if !a.previewRequests[key] {
		a.previewRequests[key] = true
		go a.loadRemotePreview(key, preview)
	}
	a.mu.Unlock()
	return queryPreview{PreviewType: "text", PreviewData: "Loading preview…", PreviewTags: preview.PreviewTags}
}

func (a *App) loadRemotePreview(path string, fallback queryPreview) {
	resolved := queryPreview{PreviewType: "text", PreviewData: "Unable to load remote preview", PreviewTags: fallback.PreviewTags}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		resolved.PreviewData = "Core returned an invalid remote preview path"
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := a.client.Get(ctx, path, &resolved)
		cancel()
		if err != nil {
			resolved = queryPreview{PreviewType: "text", PreviewData: fmt.Sprintf("Unable to load preview: %v", err), PreviewTags: fallback.PreviewTags}
		}
	}
	a.mu.Lock()
	if len(a.remotePreviews) >= 256 {
		// ponytail: Query IDs make these entries short-lived; a bounded reset avoids an LRU on the frame path.
		a.remotePreviews = map[string]queryPreview{}
		a.previewRequests = map[string]bool{path: true}
	}
	a.remotePreviews[path] = resolved
	a.mu.Unlock()
	if a.window != nil {
		_ = a.window.Invalidate()
	}
}

func normalizePreviewMetadata(preview queryPreview) queryPreview {
	if len(preview.PreviewTags) > 0 || len(preview.PreviewProperties) == 0 {
		return preview
	}
	keys := make([]string, 0, len(preview.PreviewProperties))
	for key := range preview.PreviewProperties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	preview.PreviewTags = make([]previewTag, 0, len(keys))
	for _, key := range keys {
		preview.PreviewTags = append(preview.PreviewTags, previewTag{Label: preview.PreviewProperties[key], Tooltip: key})
	}
	return preview
}

// filePreviewFor starts file inspection once and returns a stable loading state meanwhile.
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
	a.mu.Lock()
	if content, ok := a.filePreviews[path]; ok {
		a.mu.Unlock()
		return content
	}
	if !a.fileRequests[path] {
		a.fileRequests[path] = true
		go a.loadFilePreview(path, extension)
	}
	a.mu.Unlock()
	return filePreviewContent{Kind: "info", Text: "Loading file preview…"}
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

func parsePreviewImage(value string) (woxImage, bool) {
	imageType, imageData, ok := strings.Cut(value, ":")
	imageType = strings.TrimSpace(imageType)
	if !ok || imageType == "" || imageData == "" {
		return woxImage{}, false
	}
	return woxImage{ImageType: imageType, ImageData: imageData}, true
}

func decodePreviewList(value string) (previewListData, error) {
	var data previewListData
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return previewListData{}, err
	}
	return data, nil
}
