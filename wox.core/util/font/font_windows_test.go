package font

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeWindowsRegistryFontName(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"Arial (TrueType)", "Arial"},
		{"@Malgun Gothic (TrueType)", "Malgun Gothic"},
		{"苹方-简 常规体 (TrueType)", "苹方-简 常规体"},
		{"PingFang SC Regular (TrueType)", "PingFang SC Regular"},
		{"  ", ""},
	}
	for _, testCase := range cases {
		if got := sanitizeWindowsRegistryFontName(testCase.name); got != testCase.want {
			t.Fatalf("sanitizeWindowsRegistryFontName(%q) = %q, want %q", testCase.name, got, testCase.want)
		}
	}
}

func TestWindowsSystemFontFamiliesIncludeSegoeUI(t *testing.T) {
	families := getSystemFontFamilies(context.Background())
	if !containsFontFamily(families, "Segoe UI") {
		t.Fatalf("expected Segoe UI in Windows font families, got %v", families)
	}
}

func TestWindowsSystemFontFamiliesIncludeInstalledPingFang(t *testing.T) {
	if !pingFangFontInstalled() && !containsFontFamily(enumerateRegistryFontFamilies(), "苹方-简 常规体") {
		t.Skip("PingFang is not installed on this machine")
	}

	families := getSystemFontFamilies(context.Background())
	matched := filterFontFamilies(families, "PingFang", "苹方")
	t.Logf("PingFang-related families: %v", matched)
	if containsFontFamily(families, "PingFang SC") || containsFontFamily(families, "苹方-简") {
		return
	}
	t.Fatalf("installed PingFang was not enumerated as PingFang SC or 苹方-简: %v", matched)
}

func pingFangFontInstalled() bool {
	path := filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Windows", "Fonts", "PingFangSC-Regular.otf")
	_, err := os.Stat(path)
	return err == nil
}

func containsFontFamily(families []string, target string) bool {
	for _, family := range families {
		if strings.EqualFold(family, target) {
			return true
		}
	}
	return false
}

func filterFontFamilies(families []string, needles ...string) []string {
	var matched []string
	for _, family := range families {
		for _, needle := range needles {
			if strings.Contains(strings.ToLower(family), strings.ToLower(needle)) {
				matched = append(matched, family)
				break
			}
		}
	}
	return matched
}
