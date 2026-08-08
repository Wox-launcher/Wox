package appcontrol

import (
	"encoding/base64"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// NewFileMediaHandler creates the loopback handler used by embedded file previews.
func NewFileMediaHandler() http.Handler {
	return http.HandlerFunc(handleFileMedia)
}

func handleFileMedia(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	encodedPath := strings.TrimSpace(request.URL.Query().Get("path"))
	if encodedPath == "" {
		http.Error(writer, "path is empty", http.StatusBadRequest)
		return
	}

	decodedPath, err := base64.URLEncoding.DecodeString(encodedPath)
	if err != nil {
		decodedPath, err = base64.RawURLEncoding.DecodeString(encodedPath)
	}
	if err != nil {
		http.Error(writer, "path is invalid", http.StatusBadRequest)
		return
	}

	filePath := string(decodedPath)
	if filePath == "" {
		http.Error(writer, "path is empty", http.StatusBadRequest)
		return
	}
	if !filepath.IsAbs(filePath) {
		http.Error(writer, "path must be absolute", http.StatusBadRequest)
		return
	}

	stat, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(writer, request)
			return
		}
		http.Error(writer, "failed to stat file", http.StatusInternalServerError)
		return
	}
	if stat.IsDir() {
		http.Error(writer, "path is a directory", http.StatusBadRequest)
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		http.Error(writer, "failed to open file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	if contentType := previewFileMediaContentType(filePath); contentType != "" {
		writer.Header().Set("Content-Type", contentType)
	}
	writer.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(writer, request, filepath.Base(filePath), stat.ModTime(), file)
}

func previewFileMediaContentType(filePath string) string {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".pdf":
		return "application/pdf"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".m4a":
		return "audio/mp4"
	case ".aac":
		return "audio/aac"
	case ".flac":
		return "audio/flac"
	case ".ogg", ".opus":
		return "audio/ogg"
	}
	return mime.TypeByExtension(strings.ToLower(filepath.Ext(filePath)))
}
