package browser

import (
	"testing"
	"wox/common"
)

func TestBrowserIDFromExtensionRequest(t *testing.T) {
	cases := []struct {
		origin, ua, want string
	}{
		{"moz-extension://abc", "Mozilla/5.0 Firefox/155.0", BrowserIDFirefox},
		{"chrome-extension://abc", "Mozilla/5.0 Chrome/130.0", BrowserIDChrome},
		{"chrome-extension://abc", "Mozilla/5.0 Edg/130.0", BrowserIDEdge},
		{"", "Mozilla/5.0 Firefox/155.0", BrowserIDFirefox},
		{"", "Mozilla/5.0 Chrome/130.0", BrowserIDChrome},
	}
	for _, tc := range cases {
		if got := BrowserIDFromExtensionRequest(tc.origin, tc.ua); got != tc.want {
			t.Fatalf("BrowserIDFromExtensionRequest(%q, %q) = %q, want %q", tc.origin, tc.ua, got, tc.want)
		}
	}
}

func TestIconForBrowserID(t *testing.T) {
	firefoxIcon := IconForBrowserID(BrowserIDFirefox)
	chromeIcon := IconForBrowserID(BrowserIDChrome)
	unknownIcon := IconForBrowserID("unknown")
	if firefoxIcon.Hash() == chromeIcon.Hash() {
		t.Fatal("Firefox and Chrome icons must differ")
	}
	if unknownIcon.Hash() != common.ChromeIcon.Hash() {
		t.Fatal("unknown browser IDs should fall back to Chrome")
	}
}

func TestIsBrowserWindowName(t *testing.T) {
	cases := map[string]bool{
		"":                                false,
		"notepad":                         false,
		"Google Chrome":                   true,
		"firefox":                         true,
		"Mozilla Firefox":                 true,
		"GitHub - Mozilla Firefox":        true,
		"Microsoft Edge":                  true,
		"Some Page — Microsoft Edge":      true,
		"chrome.exe":                      true,
		"firefox.exe":                     true,
	}

	for name, want := range cases {
		if got := IsBrowserWindowName(name); got != want {
			t.Fatalf("IsBrowserWindowName(%q) = %v, want %v", name, got, want)
		}
	}
}
