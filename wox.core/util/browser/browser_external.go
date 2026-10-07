package browser

import (
	"errors"
	"fmt"
	"net/url"
)

// OpenExternalURL opens a web link or mail draft on the caller's GUI thread.
// Owner is an HWND on Windows, GtkWindow on Linux, and unused on macOS; it carries no UI session state.
func OpenExternalURL(rawURL string, owner uintptr) error {
	if _, err := parseExternalURL(rawURL); err != nil {
		return fmt.Errorf("unsupported external URL %q", rawURL)
	}
	// Reassembling a parsed URL can encode checkout fragments. Keep the caller's original bytes throughout dispatch.
	return openExternalURL(rawURL, owner)
}

// parseExternalURL limits desktop dispatch to the schemes supported by Wox-owned link actions.
func parseExternalURL(rawURL string) (*url.URL, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, err
	}
	if (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" {
		return parsed, nil
	}
	if parsed.Scheme == "mailto" && parsed.Opaque != "" {
		return parsed, nil
	}
	return nil, errors.New("unsupported external URL")
}
