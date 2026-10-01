package common

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestThemeNestedCapabilities covers precedence, desktop isolation, defaults, and lossless storage.
func TestThemeNestedCapabilities(t *testing.T) {
	for _, base := range []string{minimalV2Theme, `{"AppBackgroundColor":"#101010FF"}`} {
		input := strings.TrimSuffix(base, "}") + `,"linux":{"AppPaddingLeft":18,"variants":{"kde":{"AppPaddingLeft":20,"backgroundBlur":{"AppBackgroundColor":"#16161A84","AppBorderRadius":8,"AppPaddingLeft":null}},"hyprland":{"backgroundBlur":{"AppBackgroundColor":"#16161AA0"}}}}}`
		if base != minimalV2Theme {
			input = strings.ReplaceAll(input, `,"AppBorderRadius":8`, "")
		}
		var theme Theme
		if err := json.Unmarshal([]byte(input), &theme); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			desktop string
			blur    bool
			color   string
			padding int
		}{
			{"kde", false, "", 20}, {"kde", true, "#16161A84", 0},
			{"hyprland", true, "#16161AA0", 18}, {"gnome", true, "", 18},
		} {
			var capabilities []string
			if tc.blur {
				capabilities = []string{"backgroundBlur"}
			}
			resolved, err := theme.ResolveForTarget("linux", tc.desktop, capabilities...)
			if err != nil {
				t.Fatal(err)
			}
			if tc.color != "" && resolved.AppBackgroundColor != tc.color {
				t.Fatalf("%s: color %s, want %s", tc.desktop, resolved.AppBackgroundColor, tc.color)
			}
			if tc.color == "" && (resolved.AppBackgroundColor == "#16161A84" || resolved.AppBackgroundColor == "#16161AA0") {
				t.Fatal("inactive capability leaked")
			}
			padding := tc.padding
			if theme.SchemaVersion == 2 && tc.desktop == "kde" && tc.blur {
				padding = 10
			}
			if resolved.AppPaddingLeft != padding {
				t.Fatalf("%s: padding %d, want %d", tc.desktop, resolved.AppPaddingLeft, padding)
			}
			if theme.SchemaVersion == 2 && tc.desktop == "kde" && tc.blur && (resolved.AppBorderRadius == nil || *resolved.AppBorderRadius != 8) {
				t.Fatal("nested radius lost")
			}
		}
		saved, err := json.Marshal(theme)
		if err != nil {
			t.Fatal(err)
		}
		var restored Theme
		if err := json.Unmarshal(saved, &restored); err != nil {
			t.Fatal(err)
		}
		resolved, err := restored.ResolveForTarget("linux", "kde", "backgroundBlur")
		if err != nil || resolved.AppBackgroundColor != "#16161A84" {
			t.Fatalf("nested variant lost on save: %v", err)
		}
	}
}

// TestThemeRejectsInvalidInactiveCapability ensures validation reaches nested styles on other desktops.
func TestThemeRejectsInvalidInactiveCapability(t *testing.T) {
	for _, child := range []string{`null`, `[]`, `{"ThemeId":"bad"}`, `{"AppBorderRadius":-1}`, `{"AppBackgroundColor":"invalid"}`} {
		input := strings.TrimSuffix(minimalV2Theme, "}") + `,"linux":{"variants":{"kde":{"backgroundBlur":` + child + `}}}}`
		var theme Theme
		if err := json.Unmarshal([]byte(input), &theme); err == nil {
			t.Fatalf("accepted invalid nested override %s", child)
		}
	}
}
