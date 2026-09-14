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
	input := []byte(`{"SchemaVersion":2,"ThemeId":"details","ThemeName":"Details","BaseBackgroundColor":"#182020","BaseTextColor":"#E0F0E8","BaseAccentColor":"#70D6A6","ToolbarHotkeyBorderColor":"transparent","ToolbarPrimaryFontColor":"#10203080","ToolbarPrimaryHotkeyFontColor":"#20304090","ToolbarPrimaryHotkeyBackgroundColor":"transparent","ToolbarPrimaryHotkeyBorderColor":"#30405060"}`)
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
		if component.PreviewBackgroundColor == nil || component.PreviewBorderColor == nil || component.PreviewTagFontColor == nil || component.PreviewTagBackgroundColor == nil || component.PreviewTagBorderColor == nil || component.GlanceFontColor == nil || component.GlanceIconColor == nil || component.GlanceBackgroundColor == nil || component.GlanceHoverBackgroundColor == nil || component.AttentionFontColor == nil || component.AttentionIconColor == nil || component.AttentionBackgroundColor == nil || component.AttentionBorderColor == nil || component.AttentionBorderColor.A != 0 || component.AttentionHoverBackgroundColor == nil || component.AttentionHoverBorderColor == nil || component.AttentionHoverBorderColor.A != 0 || component.ToolbarHotkeyBorderColor == nil || component.ToolbarHotkeyBorderColor.A != 0 || component.ToolbarHotkeyFontColor == nil || component.ToolbarHotkeyBackgroundColor == nil || component.ActionContainerDividerColor == nil || component.ResultItemHoverBackgroundColor == nil {
			t.Fatal("missing v2 rendering color")
		}
		if component.ToolbarPrimaryFontColor == nil || component.ToolbarPrimaryFontColor.A != 128 || component.ToolbarPrimaryFontColor.R != 16 || component.ToolbarPrimaryHotkeyFontColor == nil || component.ToolbarPrimaryHotkeyFontColor.A != 144 || component.ToolbarPrimaryHotkeyBackgroundColor == nil || component.ToolbarPrimaryHotkeyBackgroundColor.A != 0 || component.ToolbarPrimaryHotkeyBorderColor == nil || component.ToolbarPrimaryHotkeyBorderColor.A != 96 {
			t.Fatal("primary toolbar colors lost in core/editor adapters")
		}
	}
	var raw map[string]any
	if err := json.Unmarshal(input, &raw); err != nil {
		t.Fatal(err)
	}
	_, values := themeEditorForm(raw)
	for _, field := range []string{"ToolbarPrimaryFontColor", "ToolbarPrimaryHotkeyFontColor", "ToolbarPrimaryHotkeyBackgroundColor", "ToolbarPrimaryHotkeyBorderColor", "PreviewBackgroundColor", "PreviewBorderColor", "PreviewTagFontColor", "PreviewTagBackgroundColor", "PreviewTagBorderColor", "GlanceFontColor", "GlanceIconColor", "GlanceBackgroundColor", "GlanceHoverBackgroundColor", "AttentionFontColor", "AttentionIconColor", "AttentionBackgroundColor", "AttentionBorderColor", "AttentionHoverBackgroundColor", "AttentionHoverBorderColor", "ActionContainerDividerColor", "ToolbarHotkeyFontColor", "ToolbarHotkeyBackgroundColor", "ToolbarHotkeyBorderColor", "ActionItemHotkeyFontColor", "ActionItemHotkeyBackgroundColor", "ActionItemHotkeyBorderColor", "ActionItemActiveHotkeyFontColor", "ActionItemActiveHotkeyBackgroundColor", "ActionItemActiveHotkeyBorderColor", "ResultItemHoverBackgroundColor"} {
		if _, exists := values[field]; !exists {
			t.Fatalf("editor missing %s", field)
		}
		if themeEditorResolvedColors(raw, values)[field] == nil {
			t.Fatalf("missing inherited swatch %s", field)
		}
	}
}
