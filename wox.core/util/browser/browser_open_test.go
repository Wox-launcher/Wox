package browser

import "testing"

func TestPrivateWindowArgs(t *testing.T) {
	cases := map[string][]string{
		BrowserIDChrome:   {"--incognito"},
		BrowserIDBrave:    {"--incognito"},
		BrowserIDChromium: {"--incognito"},
		BrowserIDOpera:    {"--incognito"},
		BrowserIDEdge:     {"--inprivate"},
		BrowserIDFirefox:  {"-private-window"},
		BrowserIDSafari:   nil,
		"":                nil,
		"unknown":         nil,
	}
	for browserID, want := range cases {
		got := privateWindowArgs(browserID)
		if !stringSlicesEqual(got, want) {
			t.Fatalf("privateWindowArgs(%q) = %#v, want %#v", browserID, got, want)
		}
	}
}

func TestURLLaunchArgs(t *testing.T) {
	if got := urlLaunchArgs(BrowserIDChrome, "https://example.com", false); !stringSlicesEqual(got, []string{"https://example.com"}) {
		t.Fatalf("normal launch args = %#v", got)
	}
	if got := urlLaunchArgs(BrowserIDEdge, "https://example.com", true); !stringSlicesEqual(got, []string{"--inprivate", "https://example.com"}) {
		t.Fatalf("edge private launch args = %#v", got)
	}
	if got := urlLaunchArgs(BrowserIDSafari, "https://example.com", true); !stringSlicesEqual(got, []string{"https://example.com"}) {
		t.Fatalf("safari private launch should fall back to a normal window, got %#v", got)
	}
}

func TestResolveLaunchBrowserID(t *testing.T) {
	if got := resolveLaunchBrowserID(BrowserIDFirefox, true); got != BrowserIDFirefox {
		t.Fatalf("explicit browser = %q", got)
	}
	if got := resolveLaunchBrowserID("system", false); got != "" {
		t.Fatalf("system browser without private = %q", got)
	}
}

func TestBrowserIDFromWindowsProgID(t *testing.T) {
	cases := map[string]string{
		"ChromeHTML":                           BrowserIDChrome,
		"MSEdgeHTM":                            BrowserIDEdge,
		"FirefoxURL-308046B0AF4A39CB":          BrowserIDFirefox,
		"BraveHTML":                            BrowserIDBrave,
		"OperaStable":                          BrowserIDOpera,
		"ChromiumHTM":                          BrowserIDChromium,
		"AppXq0fevzme2pys62n3e0fbqa7peapykr8v": "",
	}
	for progID, want := range cases {
		if got := browserIDFromWindowsProgID(progID); got != want {
			t.Fatalf("browserIDFromWindowsProgID(%q) = %q, want %q", progID, got, want)
		}
	}
}

func TestBrowserIDFromLinuxDesktopFile(t *testing.T) {
	cases := map[string]string{
		"google-chrome.desktop":      BrowserIDChrome,
		"com.google.Chrome.desktop":  BrowserIDChrome,
		"chromium-browser.desktop":   BrowserIDChromium,
		"microsoft-edge.desktop":     BrowserIDEdge,
		"firefox_firefox.desktop":    BrowserIDFirefox,
		"brave-browser.desktop":      BrowserIDBrave,
		"opera.desktop":              BrowserIDOpera,
		"org.gnome.Epiphany.desktop": "",
	}
	for desktop, want := range cases {
		if got := browserIDFromLinuxDesktopFile(desktop); got != want {
			t.Fatalf("browserIDFromLinuxDesktopFile(%q) = %q, want %q", desktop, got, want)
		}
	}
}

func TestBrowserIDFromMacAppPath(t *testing.T) {
	if got := browserIDFromMacAppPath("/Applications/Google Chrome.app"); got != BrowserIDChrome {
		t.Fatalf("chrome app path = %q", got)
	}
	if got := browserIDFromMacAppPath("/Applications/Safari.app/"); got != BrowserIDSafari {
		t.Fatalf("safari app path = %q", got)
	}
}

func TestBrowserIDFromLaunchServicesJSON(t *testing.T) {
	data := []byte(`{
		"LSHandlers": [
			{"LSHandlerURLScheme": "https", "LSHandlerRoleAll": "com.apple.Safari"},
			{"LSHandlerURLScheme": "https", "LSHandlerRoleAll": "com.google.Chrome"},
			{"LSHandlerURLScheme": "mailto", "LSHandlerRoleAll": "com.apple.mail"}
		]
	}`)
	if got := browserIDFromLaunchServicesJSON(data); got != BrowserIDChrome {
		t.Fatalf("last https handler = %q, want chrome", got)
	}
}

func stringSlicesEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
