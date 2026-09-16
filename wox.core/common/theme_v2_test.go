package common

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

const minimalV2Theme = `{"SchemaVersion":2,"ThemeId":"test-v2","ThemeName":"Test","BaseBackgroundColor":"#182020B8","BaseTextColor":"#E0F0E8","BaseAccentColor":"#70D6A6"}`

// TestToolbarPrimaryThemeInheritance preserves sparse saves, per-platform inheritance and transparent overrides.
func TestToolbarPrimaryThemeInheritance(t *testing.T) {
	input := strings.TrimSuffix(minimalV2Theme, "}") + `,"ToolbarFontColor":"#12345680","ToolbarHotkeyFontColor":"#23456790","ToolbarHotkeyBackgroundColor":"#34567860","ToolbarHotkeyBorderColor":"#45678950","windows":{"ToolbarFontColor":"#ABCDEF80","ToolbarPrimaryHotkeyBorderColor":"transparent"}}`
	var theme Theme
	if err := json.Unmarshal([]byte(input), &theme); err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"windows", "macos", "linux"} {
		resolved, err := theme.ResolveForTarget(platform, "")
		if err != nil {
			t.Fatal(err)
		}
		colors := resolved.ResolvedColors()
		for primary, normal := range map[string]string{"ToolbarPrimaryFontColor": "ToolbarFontColor", "ToolbarPrimaryHotkeyFontColor": "ToolbarHotkeyFontColor", "ToolbarPrimaryHotkeyBackgroundColor": "ToolbarHotkeyBackgroundColor", "ToolbarPrimaryHotkeyBorderColor": "ToolbarHotkeyBorderColor"} {
			want := colors[normal]
			if platform == "windows" && primary == "ToolbarPrimaryHotkeyBorderColor" {
				want = "#00000000"
			}
			if colors[primary] != want {
				t.Fatalf("%s/%s = %s, want %s", platform, primary, colors[primary], want)
			}
		}
	}
	encoded, err := json.Marshal(theme)
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &saved); err != nil {
		t.Fatal(err)
	}
	for key := range saved {
		if strings.HasPrefix(key, "ToolbarPrimary") {
			t.Fatalf("save materialized inherited field %s", key)
		}
	}
	var restored Theme
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	resolved, err := restored.ResolveForTarget("windows", "")
	if err != nil || resolved.ResolvedColors()["ToolbarPrimaryHotkeyBorderColor"] != "#00000000" {
		t.Fatal("save lost explicit transparent platform override")
	}
}

// TestBuiltinHotkeyHierarchy validates shipped palettes without dimming selected shortcut text.
func TestBuiltinHotkeyHierarchy(t *testing.T) {
	for _, name := range []string{"dark", "light", "glass", "jade"} {
		data, err := os.ReadFile("../resource/themes/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var theme Theme
		if err := json.Unmarshal(data, &theme); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		colors := theme.ResolvedColors()
		if !strings.HasSuffix(colors["ToolbarHotkeyBorderColor"], "4D") || !strings.HasSuffix(colors["ActionItemHotkeyBorderColor"], "4D") || !strings.HasSuffix(colors["ActionItemActiveHotkeyBorderColor"], "66") || colors["ActionItemActiveHotkeyBackgroundColor"] != "#00000000" {
			t.Fatalf("%s: keycap chrome should remain secondary", name)
		}
		if colors["ActionItemActiveHotkeyFontColor"] != theme.ActionItemActiveFontColor {
			t.Fatalf("%s: selected shortcut text must retain the selected label contrast", name)
		}
	}
}

// TestThemeV2ContentPanel keeps sparse defaults, platform overrides and material selection independent.
func TestThemeV2ContentPanel(t *testing.T) {
	for _, extra := range []string{"", `,"AppContentInset":null,"AppContentBackgroundColor":null,"AppContentBorderRadius":null`, `,"AppContentInset":0,"AppContentBackgroundColor":"transparent","AppContentBorderRadius":0`} {
		var theme Theme
		if err := json.Unmarshal([]byte(strings.TrimSuffix(minimalV2Theme, "}")+extra+"}"), &theme); err != nil {
			t.Fatal(err)
		}
		if theme.AppContentInset != 0 || theme.AppContentBorderRadius != 0 || theme.AppContentBackgroundColor != "#00000000" || theme.UsesCustomWindowChrome() {
			t.Fatal("content defaults changed existing window appearance")
		}
		encoded, err := json.Marshal(theme)
		if err != nil {
			t.Fatal(err)
		}
		if extra == "" && strings.Contains(string(encoded), "AppContent") {
			t.Fatal("sparse save materialized content defaults")
		}
	}
	input := strings.TrimSuffix(minimalV2Theme, "}") + `,"AppContentInset":12,"AppContentBackgroundColor":"#203B4ECC","AppContentBorderRadius":8,"windows":{"AppContentInset":16,"variants":{"win11":{"AppContentBorderRadius":0}}}}`
	var theme Theme
	if err := json.Unmarshal([]byte(input), &theme); err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"windows", "macos", "linux"} {
		resolved, err := theme.ResolveForTarget(platform, "win11")
		if err != nil {
			t.Fatal(err)
		}
		inset, radius := 12, 8
		if platform == "windows" {
			inset, radius = 16, 0
		}
		if resolved.AppContentInset != inset || resolved.AppContentBorderRadius != radius || resolved.AppContentBackgroundColor != "#203B4ECC" || resolved.UsesCustomWindowChrome() {
			t.Fatalf("incorrect %s content panel: %+v", platform, resolved)
		}
	}
	encoded, err := json.Marshal(theme)
	if err != nil {
		t.Fatal(err)
	}
	var restored Theme
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(theme, restored) {
		t.Fatal("content panel round trip changed authored values")
	}
	for _, extra := range []string{`,"AppContentInset":-1`, `,"AppContentInset":1.5`, `,"AppContentBorderRadius":-1`, `,"AppContentBackgroundColor":"invalid"`, `,"linux":{"AppContentInset":-1}`} {
		if err := json.Unmarshal([]byte(strings.TrimSuffix(minimalV2Theme, "}")+extra+"}"), &restored); err == nil {
			t.Fatalf("accepted invalid content style %s", extra)
		}
	}
}

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

// TestThemeV2CustomWindowChromeFollowsAuthoredOutline keeps resolved color defaults from disabling system material.
func TestThemeV2CustomWindowChromeFollowsAuthoredOutline(t *testing.T) {
	var theme Theme
	if err := json.Unmarshal([]byte(minimalV2Theme), &theme); err != nil {
		t.Fatal(err)
	}
	if theme.UsesCustomWindowChrome() {
		t.Fatal("resolved AppBorderColor default selected custom window chrome")
	}
	for _, extra := range []string{`,"AppBorderColor":"#4FAE85"`, `,"AppBorderWidth":0`, `,"AppBorderRadius":0`} {
		var authored Theme
		if err := json.Unmarshal([]byte(strings.TrimSuffix(minimalV2Theme, "}")+extra+`}`), &authored); err != nil {
			t.Fatal(err)
		}
		if !authored.UsesCustomWindowChrome() {
			t.Fatalf("authored %s did not select custom window chrome", extra)
		}
	}
	input := strings.TrimSuffix(minimalV2Theme, "}") + `,"macos":{"AppBorderRadius":18}}`
	if err := json.Unmarshal([]byte(input), &theme); err != nil {
		t.Fatal(err)
	}
	windows, err := theme.ResolveForTarget("windows", "")
	if err != nil {
		t.Fatal(err)
	}
	if windows.UsesCustomWindowChrome() {
		t.Fatal("macOS outline disabled Windows material")
	}
	mac, err := theme.ResolveForTarget("darwin", "")
	if err != nil {
		t.Fatal(err)
	}
	if !mac.UsesCustomWindowChrome() {
		t.Fatal("macOS outline kept system material")
	}
}

// TestThemeV2PlatformNullClearsInheritedWindowChrome lets a variant restore system
// material after a parent AppBorder* outline. Explicit zero still selects custom chrome.
func TestThemeV2PlatformNullClearsInheritedWindowChrome(t *testing.T) {
	input := strings.TrimSuffix(minimalV2Theme, "}") + `,"linux":{"AppBorderRadius":8,"AppPaddingLeft":18,"variants":{"hyprland":{"AppBorderRadius":null,"AppPaddingLeft":null}}}}`
	var theme Theme
	if err := json.Unmarshal([]byte(input), &theme); err != nil {
		t.Fatal(err)
	}
	linux, err := theme.ResolveForTarget("linux", "")
	if err != nil {
		t.Fatal(err)
	}
	if !linux.UsesCustomWindowChrome() || linux.AppBorderRadius == nil || *linux.AppBorderRadius != 8 || linux.AppPaddingLeft != 18 {
		t.Fatalf("linux should keep the authored outline: %#v", linux)
	}
	hyprland, err := theme.ResolveForTarget("linux", "hyprland")
	if err != nil {
		t.Fatal(err)
	}
	if hyprland.UsesCustomWindowChrome() || hyprland.AppBorderRadius != nil {
		t.Fatal("hyprland null AppBorderRadius should restore system material")
	}
	if hyprland.AppPaddingLeft != 10 {
		t.Fatalf("hyprland null AppPaddingLeft = %d, want default 10", hyprland.AppPaddingLeft)
	}
	encoded, err := json.Marshal(theme)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"AppBorderRadius":null`) {
		t.Fatalf("save dropped the hyprland null chrome reset: %s", encoded)
	}
	zero := strings.TrimSuffix(minimalV2Theme, "}") + `,"linux":{"AppBorderRadius":8,"variants":{"hyprland":{"AppBorderRadius":0}}}}`
	if err := json.Unmarshal([]byte(zero), &theme); err != nil {
		t.Fatal(err)
	}
	hyprland, err = theme.ResolveForTarget("linux", "hyprland")
	if err != nil {
		t.Fatal(err)
	}
	if !hyprland.UsesCustomWindowChrome() || hyprland.AppBorderRadius == nil || *hyprland.AppBorderRadius != 0 {
		t.Fatal("explicit zero AppBorderRadius must keep custom chrome")
	}
}

// TestBuiltinThemesWindowChrome keeps ordinary themes on system material and outline themes self-drawn.
func TestBuiltinThemesWindowChrome(t *testing.T) {
	for _, name := range []string{"auto", "dark", "light", "glass"} {
		data, err := os.ReadFile("../resource/themes/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var theme Theme
		if err := json.Unmarshal(data, &theme); err != nil {
			t.Fatal(err)
		}
		if theme.UsesCustomWindowChrome() {
			t.Fatalf("%s selected custom window chrome", name)
		}
		if name == "auto" {
			continue
		}
		linux, err := theme.ResolveForTarget("linux", "")
		if err != nil {
			t.Fatal(err)
		}
		if !linux.UsesCustomWindowChrome() {
			t.Fatalf("%s linux dropped the rounded fallback chrome", name)
		}
		hyprland, err := theme.ResolveForTarget("linux", "hyprland")
		if err != nil {
			t.Fatal(err)
		}
		if hyprland.UsesCustomWindowChrome() || hyprland.AppBorderRadius != nil {
			t.Fatalf("%s hyprland disabled compositor material", name)
		}
	}
	for _, name := range []string{"jade"} {
		data, err := os.ReadFile("../resource/themes/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var theme Theme
		if err := json.Unmarshal(data, &theme); err != nil {
			t.Fatal(err)
		}
		if !theme.UsesCustomWindowChrome() {
			t.Fatalf("%s kept system window material", name)
		}
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
