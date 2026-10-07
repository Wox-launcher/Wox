package webview

import (
	"errors"
	"net/url"
)

// OpenPageInBrowser keeps the embedded page's HTTP-only policy separate from desktop URL dispatch.
func OpenPageInBrowser(rawURL string, open func(string) error) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errors.New("webview: current page cannot be opened in a browser")
	}
	return open(rawURL)
}
