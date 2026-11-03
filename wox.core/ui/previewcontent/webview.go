// Package previewcontent defines content shared by preview surfaces.
package previewcontent

import (
	"encoding/json"
	"errors"
	"strings"
)

// WebView contains a WebView document and its optional presentation settings.
type WebView struct {
	URL           string `json:"url"`
	HTML          string `json:"html"`
	InjectCSS     string `json:"injectCss"`
	UserAgent     string `json:"userAgent"`
	CacheDisabled bool   `json:"cacheDisabled"`
	CacheKey      string `json:"cacheKey"`
}

// DecodeWebView accepts both the structured preview payload and legacy URLs.
func DecodeWebView(raw string) (WebView, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return WebView{}, errors.New("preview data is empty")
	}
	if !strings.HasPrefix(trimmed, "{") {
		return WebView{URL: trimmed}, nil
	}
	var data WebView
	if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
		return WebView{}, err
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal([]byte(trimmed), &fields)
	_, hasMode := fields["updateMode"]
	_, hasScript := fields["script"]
	if hasMode || hasScript {
		return WebView{}, errors.New("script updates are only valid in UpdateResult")
	}
	if strings.TrimSpace(data.URL) == "" && data.HTML == "" {
		return WebView{}, errors.New("preview requires a URL or HTML")
	}
	return data, nil
}
