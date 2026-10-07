//go:build windows

package browser

import (
	"errors"
	"strings"
	"testing"
)

// TestExternalURLLaunchPreservesRawLink covers checkout fragments, mail routing, and owner-aware fallback without launching an app.
func TestExternalURLLaunchPreservesRawLink(t *testing.T) {
	checkout := "https://checkout.stripe.com/c/pay/cs_live_" + strings.Repeat("abc", 2048) + "#fidk+Unmodified%2Fpart"
	mail := "mailto:billing@woxlauncher.com?subject=Billing+help"
	for _, test := range []struct {
		name       string
		url        string
		browserID  string
		browserErr error
		wantShell  bool
	}{
		{name: "checkout", url: checkout, browserID: BrowserIDChrome},
		{name: "unknown browser", url: checkout, wantShell: true},
		{name: "failed browser", url: checkout, browserID: BrowserIDChrome, browserErr: errors.New("launch failed"), wantShell: true},
		{name: "mail", url: mail, browserID: BrowserIDChrome, wantShell: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			browserCalls, shellCalls := 0, 0
			const owner = uintptr(0x1234)
			shellErr := errors.New("shell failed")
			err := launchExternalURL(test.url, owner, func() string { return test.browserID }, func(gotURL, gotID string) error {
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
			if test.url == mail && browserCalls != 0 {
				t.Fatal("mail draft was sent to a web browser")
			}
		})
	}
}
