package webview

import (
	"errors"
	"strings"
	"testing"
)

// TestOpenPageInBrowser checks that page policy and desktop dispatch preserve checkout URLs.
func TestOpenPageInBrowser(t *testing.T) {
	rawURL := "https://checkout.stripe.com/c/pay/" + strings.Repeat("a", 7000) + "#fidkdWxOYHwnPyd1blpxYHZxWjA0+/%3D"
	wantErr := errors.New("desktop launch failed")
	err := OpenPageInBrowser(rawURL, func(got string) error {
		if got != rawURL {
			t.Fatal("page URL changed before desktop dispatch")
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("launch error = %v, want %v", err, wantErr)
	}
	for _, rawURL := range []string{"", "about:blank", "file:///tmp/page.html", "data:text/html,hello", "mailto:user@example.com", "https:///missing-host"} {
		t.Run(rawURL, func(t *testing.T) {
			if err := OpenPageInBrowser(rawURL, func(string) error {
				t.Fatal("non-web page reached desktop dispatch")
				return nil
			}); err == nil {
				t.Fatal("expected unsupported page error")
			}
		})
	}
}
