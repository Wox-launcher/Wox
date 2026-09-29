package locale

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TestWindowsLocaleAPI verifies the native call on the host without launching a shell.
func TestWindowsLocaleAPI(t *testing.T) {
	var name [85]uint16
	n, _, err := getUserDefaultLocaleName.Call(uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
	if n == 0 {
		t.Fatalf("GetUserDefaultLocaleName: %v", err)
	}
	localeName := windows.UTF16ToString(name[:])
	if _, _, err := parseWindowsLocale(localeName); err != nil {
		t.Fatalf("parse native locale %q: %v", localeName, err)
	}
	t.Logf("native locale: %s", localeName)
}

// TestParseWindowsLocale covers regional, scripted, and invalid locale names.
func TestParseWindowsLocale(t *testing.T) {
	for _, tc := range []struct{ name, lang, region string }{
		{"en-US", "en", "US"},
		{"pt-BR", "pt", "BR"},
		{"zh-CN", "zh", "CN"},
		{"zh-Hans-CN", "zh", "CN"},
		{"sr-Latn-RS", "sr", "RS"},
		{"zh-CN_stroke", "zh", "CN"},
		{"zh-TW_pronun", "zh", "TW"},
		{"es-ES_tradnl", "es", "ES"},
		{"de-DE_phoneb", "de", "DE"},
		{"hu-HU_technl", "hu", "HU"},
		{"", "", ""},
		{"en", "", ""},
		{"invalid!", "", ""},
		{"en-US-!", "", ""},
	} {
		lang, region, err := parseWindowsLocale(tc.name)
		if lang != tc.lang || region != tc.region || (err != nil) != (tc.lang == "") {
			t.Errorf("parseWindowsLocale(%q) = %q, %q, %v", tc.name, lang, region, err)
		}
	}
}
