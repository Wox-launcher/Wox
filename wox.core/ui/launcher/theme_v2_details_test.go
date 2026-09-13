package launcher

import (
	"encoding/json"
	"testing"
	"wox/common"
)

// TestV2ContentPanelReachesRenderer covers both core delivery and editor preview parsing.
func TestV2ContentPanelReachesRenderer(t *testing.T) {
	input := []byte(`{"SchemaVersion":2,"ThemeId":"content","ThemeName":"Content","BaseBackgroundColor":"#182020","BaseTextColor":"#E0F0E8","BaseAccentColor":"#70D6A6","AppContentInset":12,"AppContentBackgroundColor":"#203B4ECC","AppContentBorderRadius":8}`)
	var theme common.Theme
	if err := json.Unmarshal(input, &theme); err != nil {
		t.Fatal(err)
	}
	var preview themeData
	if err := json.Unmarshal(input, &preview); err != nil {
		t.Fatal(err)
	}
	for _, data := range []themeData{fromCoreTheme(theme), preview} {
		component := paletteForTheme(data).componentTheme()
		if component.AppContentInset != 12 || component.AppContentBorderRadius != 8 || component.AppContentBackground.A != 204 || component.AppWindowChrome {
			t.Fatalf("content panel lost in adapter: %+v", component)
		}
	}
}

// TestV2DetailColorsReachEditorAndRenderer verifies all optional fields survive the adapters.
func TestV2DetailColorsReachEditorAndRenderer(t *testing.T) {
	input := []byte(`{"SchemaVersion":2,"ThemeId":"details","ThemeName":"Details","BaseBackgroundColor":"#182020","BaseTextColor":"#E0F0E8","BaseAccentColor":"#70D6A6","ToolbarHotkeyBorderColor":"transparent"}`)
	var theme common.Theme
	if err := json.Unmarshal(input, &theme); err != nil {
		t.Fatal(err)
	}
	var preview themeData
	if err := json.Unmarshal(input, &preview); err != nil {
		t.Fatal(err)
	}
	for _, data := range []themeData{fromCoreTheme(theme), preview} {
		component := paletteForTheme(data).componentTheme()
		if component.PreviewBackgroundColor == nil || component.PreviewBorderColor == nil || component.PreviewTagFontColor == nil || component.PreviewTagBackgroundColor == nil || component.PreviewTagBorderColor == nil || component.GlanceFontColor == nil || component.GlanceIconColor == nil || component.GlanceBackgroundColor == nil || component.GlanceHoverBackgroundColor == nil || component.ToolbarHotkeyBorderColor == nil || component.ToolbarHotkeyBorderColor.A != 0 || component.ToolbarHotkeyFontColor == nil || component.ToolbarHotkeyBackgroundColor == nil || component.ActionContainerDividerColor == nil || component.ResultItemHoverBackgroundColor == nil {
			t.Fatal("missing v2 rendering color")
		}
	}
	var raw map[string]any
	if err := json.Unmarshal(input, &raw); err != nil {
		t.Fatal(err)
	}
	_, values := themeEditorForm(raw)
	for _, field := range []string{"PreviewBackgroundColor", "PreviewBorderColor", "PreviewTagFontColor", "PreviewTagBackgroundColor", "PreviewTagBorderColor", "GlanceFontColor", "GlanceIconColor", "GlanceBackgroundColor", "GlanceHoverBackgroundColor", "ActionContainerDividerColor", "ToolbarHotkeyFontColor", "ToolbarHotkeyBackgroundColor", "ToolbarHotkeyBorderColor", "ActionItemHotkeyFontColor", "ActionItemHotkeyBackgroundColor", "ActionItemHotkeyBorderColor", "ActionItemActiveHotkeyFontColor", "ActionItemActiveHotkeyBackgroundColor", "ActionItemActiveHotkeyBorderColor", "ResultItemHoverBackgroundColor"} {
		if _, exists := values[field]; !exists {
			t.Fatalf("editor missing %s", field)
		}
		if themeEditorResolvedColors(raw, values)[field] == nil {
			t.Fatalf("missing inherited swatch %s", field)
		}
	}
}
