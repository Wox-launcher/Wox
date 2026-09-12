package common

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

const minimalV2Theme = `{"SchemaVersion":2,"ThemeId":"test-v2","ThemeName":"Test","BaseBackgroundColor":"#182020B8","BaseTextColor":"#E0F0E8","BaseAccentColor":"#70D6A6"}`

// TestThemeV2OptionalValuesRoundTrip verifies that resolving never materializes absent authored fields.
func TestThemeV2OptionalValuesRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		extra   string
		padding int
	}{{"", 10}, {`,"AppPaddingLeft":null`, 10}, {`,"AppPaddingLeft":0`, 0}, {`,"AppPaddingLeft":18`, 18}} {
		var theme Theme
		input := strings.TrimSuffix(minimalV2Theme, "}") + tc.extra + `}`
		if err := json.Unmarshal([]byte(input), &theme); err != nil {
			t.Fatal(err)
		}
		if theme.AppPaddingLeft != tc.padding || theme.QueryBoxFontColor != "#E0F0E8FF" || theme.QueryBoxCursorColor != "#70D6A6FF" {
			t.Fatalf("invalid resolved theme: %#v", theme)
		}
		encoded, err := json.Marshal(theme)
		if err != nil {
			t.Fatal(err)
		}
		var raw map[string]any
		if err := json.Unmarshal(encoded, &raw); err != nil {
			t.Fatal(err)
		}
		if _, ok := raw["QueryBoxFontColor"]; ok {
			t.Fatal("inherited text color was written")
		}
		_, hasPadding := raw["AppPaddingLeft"]
		if hasPadding != (tc.padding != 10) {
			t.Fatalf("unexpected saved padding: %s", encoded)
		}
		var restored Theme
		if err := json.Unmarshal(encoded, &restored); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(theme, restored) {
			t.Fatal("round trip changed theme")
		}
	}
}

// TestThemeV2PlatformResolution preserves the authored source and computes dependent defaults after overrides.
func TestThemeV2PlatformResolution(t *testing.T) {
	input := strings.TrimSuffix(minimalV2Theme, "}") + `,"windows":{"BaseAccentColor":"#112233","AppPaddingLeft":0,"variants":{"win11":{"BaseTextColor":"#ABCDEF","ToolbarBorderWidth":0}}}}`
	var theme Theme
	if err := json.Unmarshal([]byte(input), &theme); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(theme)
	resolved, err := theme.ResolveForTarget("windows", "win11")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.AppPaddingLeft != 0 || resolved.QueryBoxCursorColor != "#112233FF" || resolved.ResultItemActiveIndicatorColor != "#112233FF" || resolved.ResultItemTitleColor != "#ABCDEFFF" || *resolved.ToolbarBorderWidth != 0 {
		t.Fatalf("wrong platform defaults: %#v", resolved)
	}
	after, _ := json.Marshal(resolved)
	if string(before) != string(after) {
		t.Fatal("platform resolution changed saved source")
	}
	mac, err := theme.ResolveForTarget("darwin", "")
	if err != nil {
		t.Fatal(err)
	}
	if mac.AppPaddingLeft != 10 || mac.QueryBoxCursorColor != "#70D6A6FF" {
		t.Fatal("Windows override affected macOS")
	}
}

// TestThemeV2RejectsInvalidValues covers strict new-format validation without changing legacy parsing.
func TestThemeV2RejectsInvalidValues(t *testing.T) {
	for _, extra := range []string{`,"ResultItemActiveBorderLeftWidth":3`, `,"ResultItemActiveIndicatorWidth":-1`, `,"ResultItemActiveIndicatorInsetTop":1.5`, `,"QueryBoxBorderBottomWidth":-1`, `,"AppPaddingLeft":-1`, `,"AppPaddingLeft":1.5`, `,"ResultItemTitleColor":""`, `,"ToolbarBorderColor":"rgba(0,0,0,2)"`, `,"windows":{"variants":{"win11":{"ActionContainerBorderWidth":-1}}}`} {
		var theme Theme
		if err := json.Unmarshal([]byte(strings.TrimSuffix(minimalV2Theme, "}")+extra+`}`), &theme); err == nil {
			t.Fatalf("accepted invalid %s", extra)
		}
	}
	for _, input := range []string{strings.Replace(minimalV2Theme, `"SchemaVersion":2`, `"SchemaVersion":99`, 1), strings.Replace(minimalV2Theme, `"BaseTextColor":"#E0F0E8"`, `"BaseTextColor":null`, 1)} {
		var theme Theme
		if err := json.Unmarshal([]byte(input), &theme); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
	var theme Theme
	if err := json.Unmarshal([]byte(strings.TrimSuffix(minimalV2Theme, "}")+`,"ResultItemActiveIndicatorColor":"transparent","ActionContainerBorderRadius":0}`), &theme); err != nil {
		t.Fatal(err)
	}
	if theme.ResultItemActiveIndicatorColor != "#00000000" || *theme.ActionContainerBorderRadius != 0 {
		t.Fatal("explicit zero or transparent was replaced")
	}
}

// TestLegacyThemeDefinitionIsolation verifies saved v1 fixtures stay on the original parsing and serialization path.
func TestLegacyThemeDefinitionIsolation(t *testing.T) {
	for _, name := range []string{"auto", "dark", "light", "glass"} {
		data, err := os.ReadFile("testdata/theme_v1/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var theme Theme
		if err := json.Unmarshal(data, &theme); err != nil {
			t.Fatal(err)
		}
		if theme.HasAuthoredStyles() || theme.SchemaVersion != 1 {
			t.Fatal("legacy theme upgraded implicitly")
		}
		var legacy ThemeSchemaV1
		if err := json.Unmarshal(data, &legacy); err != nil {
			t.Fatal(err)
		}
		legacy.SchemaVersion = 1
		want, err := json.Marshal(legacy)
		if err != nil {
			t.Fatal(err)
		}
		got, err := json.Marshal(theme)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Fatalf("%s legacy serialization changed", name)
		}
	}
	var theme Theme
	if err := json.Unmarshal([]byte(`{"ResultItemActiveBorderLeft":"4","AppPaddingLeft":0}`), &theme); err != nil {
		t.Fatal(err)
	}
	if theme.ResultItemActiveBorderLeftWidth != 4 || theme.AppPaddingLeft != 0 || theme.AppPaddingTop != 0 {
		t.Fatal("legacy alias/zero/missing behavior changed")
	}
}
