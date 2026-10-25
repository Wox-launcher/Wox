//go:build windows

package browser

import (
	"errors"
	"strings"
	"testing"
)

// TestExternalURLLaunchPreservesRawLink covers system routing, long checkout fragments, and owner-aware fallback without launching an app.
func TestExternalURLLaunchPreservesRawLink(t *testing.T) {
	checkout := "https://checkout.stripe.com/c/pay/cs_live_" + strings.Repeat("abc", 2048) + "#fidk+Unmodified%2Fpart"
	mail := "mailto:billing@woxlauncher.com?subject=Billing+help"
	webPrefix := "https://example.com/"
	for _, test := range []struct {
		name       string
		url        string
		browserID  string
		browserErr error
		wantDetect bool
		wantShell  bool
	}{
		{name: "https ignores stale browser", url: "https://github.com/Wox-launcher/Wox", browserID: BrowserIDChrome, wantShell: true},
		{name: "http ignores stale browser", url: "http://www.wox.one", browserID: BrowserIDChrome, wantShell: true},
		{name: "short checkout", url: "https://checkout.stripe.com/c/pay/cs_live_abc#fidk+Unmodified%2Fpart", browserID: BrowserIDChrome, wantShell: true},
		{name: "shell length boundary", url: webPrefix + strings.Repeat("a", 2048-len(webPrefix)), browserID: BrowserIDChrome, wantShell: true},
		{name: "above shell length boundary", url: webPrefix + strings.Repeat("a", 2049-len(webPrefix)), browserID: BrowserIDChrome, wantDetect: true},
		{name: "checkout", url: checkout, browserID: BrowserIDChrome, wantDetect: true},
		{name: "unknown browser", url: checkout, wantDetect: true, wantShell: true},
		{name: "failed browser", url: checkout, browserID: BrowserIDChrome, browserErr: errors.New("launch failed"), wantDetect: true, wantShell: true},
		{name: "mail", url: mail, browserID: BrowserIDChrome, wantShell: true},
		{name: "long mail", url: mail + strings.Repeat("a", 2048), browserID: BrowserIDChrome, wantShell: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			detectCalls, browserCalls, shellCalls := 0, 0, 0
			const owner = uintptr(0x1234)
			shellErr := errors.New("shell failed")
			err := launchExternalURL(test.url, owner, func() string {
				detectCalls++
				return test.browserID
			}, func(gotURL, gotID string) error {
				browserCalls++
				if gotURL != test.url || gotID != test.browserID {
					t.Fatalf("browser arguments changed: %q %q", gotURL, gotID)
				}
				return test.browserErr
			}, func(gotURL string, gotOwner uintptr) error {
				shellCalls++
				if gotURL != test.url || gotOwner != owner {
					t.Fatalf("shell arguments changed: %q %#x", gotURL, gotOwner)
				}
				return shellErr
			})
			if test.wantShell {
				if shellCalls != 1 || !errors.Is(err, shellErr) {
					t.Fatalf("fallback calls=%d error=%v", shellCalls, err)
				}
			} else if shellCalls != 0 || err != nil {
				t.Fatalf("successful browser launch fell back: calls=%d error=%v", shellCalls, err)
			}
			wantDetectCalls, wantBrowserCalls := 0, 0
			if test.wantDetect {
				wantDetectCalls = 1
				if test.browserID != "" {
					wantBrowserCalls = 1
				}
			}
			if detectCalls != wantDetectCalls || browserCalls != wantBrowserCalls {
				t.Fatalf("detect calls=%d browser calls=%d, want %d and %d", detectCalls, browserCalls, wantDetectCalls, wantBrowserCalls)
			}
		})
	}
}
