package common

import (
	"encoding/json"
	"image/color"
	"strings"
	"testing"
)

// TestThemeStatusColorsRoundtrip covers legacy and v2 overrides without persisting defaults.
func TestThemeStatusColorsRoundtrip(t *testing.T) {
	for _, source := range []string{`{"ThemeName":"Legacy"}`, minimalV2Theme} {
		var theme Theme
		input := strings.TrimSuffix(source, "}") + `,"SuccessColor":"#12345680","WarningColor":"#abcdef","WarningTextColor":"#102030","ErrorColor":"transparent"}`
		if err := json.Unmarshal([]byte(input), &theme); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(theme)
		if err != nil {
			t.Fatal(err)
		}
		var saved Theme
		if err := json.Unmarshal(encoded, &saved); err != nil {
			t.Fatal(err)
		}
		for _, dark := range []bool{false, true} {
			got := saved.StatusColors(dark)
			if got.Success != (color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0x80}) || got.Warning != (color.NRGBA{R: 0xab, G: 0xcd, B: 0xef, A: 255}) || got.WarningText != (color.NRGBA{R: 0x10, G: 0x20, B: 0x30, A: 255}) || got.Error != (color.NRGBA{}) {
				t.Fatalf("lost status overrides: %+v", got)
			}
		}
		if err := json.Unmarshal([]byte(source), &theme); err != nil {
			t.Fatal(err)
		}
		if theme.StatusColors(true).Success == theme.StatusColors(false).Success {
			t.Fatal("default success color ignores surface appearance")
		}
		encoded, _ = json.Marshal(theme)
		if strings.Contains(string(encoded), "SuccessColor") {
			t.Fatal("default palette was written into source")
		}
	}
}

// TestThemeStatusColorsVariants exercises inheritance, explicit clearing and inactive validation.
func TestThemeStatusColorsVariants(t *testing.T) {
	input := strings.TrimSuffix(minimalV2Theme, "}") + `,"SuccessColor":"#123456","windows":{"SuccessColor":null,"ErrorColor":"#abcdef"}}`
	var theme Theme
	if err := json.Unmarshal([]byte(input), &theme); err != nil {
		t.Fatal(err)
	}
	resolved, err := theme.ResolveForTarget("windows", "win11")
	if err != nil {
		t.Fatal(err)
	}
	if got := resolved.StatusColors(true); got.Success != (Theme{}).StatusColors(true).Success || got.Error != (color.NRGBA{R: 0xab, G: 0xcd, B: 0xef, A: 255}) {
		t.Fatalf("variant statuses: %+v", got)
	}
	if err := json.Unmarshal([]byte(strings.Replace(input, `"ErrorColor":"#abcdef"`, `"ErrorColor":"invalid"`, 1)), &theme); err == nil {
		t.Fatal("accepted invalid inactive status override")
	}
}
