package plugin

import (
	"encoding/json"
	"errors"
	"strings"
)

// WebViewScriptUpdate identifies a one-shot UpdateResult command, never document content.
func WebViewScriptUpdate(preview *WoxPreview) (script string, command bool, err error) {
	if preview == nil || preview.PreviewType != WoxPreviewTypeWebView || !strings.HasPrefix(strings.TrimSpace(preview.PreviewData), "{") {
		return
	}
	var data map[string]json.RawMessage
	if json.Unmarshal([]byte(preview.PreviewData), &data) != nil {
		return
	}
	mode, hasMode := data["updateMode"]
	_, hasScript := data["script"]
	if !hasMode && !hasScript {
		return
	}
	command = true
	var value string
	if json.Unmarshal(mode, &value) != nil || value != "script" {
		return "", true, errors.New("unsupported WebView updateMode")
	}
	if _, ok := data["html"]; ok {
		return "", true, errors.New("script update cannot include html")
	}
	if _, ok := data["url"]; ok {
		return "", true, errors.New("script update cannot include url")
	}
	if json.Unmarshal(data["script"], &script) != nil || strings.TrimSpace(script) == "" {
		return "", true, errors.New("script update requires nonempty script")
	}
	return
}

// IsInlineWebViewPreview excludes URL documents and one-shot commands.
func IsInlineWebViewPreview(preview WoxPreview) bool {
	if preview.PreviewType != WoxPreviewTypeWebView {
		return false
	}
	if _, command, _ := WebViewScriptUpdate(&preview); command {
		return false
	}
	var data struct {
		HTML string `json:"html"`
		URL  string `json:"url"`
	}
	return json.Unmarshal([]byte(preview.PreviewData), &data) == nil && data.HTML != "" && data.URL == ""
}
