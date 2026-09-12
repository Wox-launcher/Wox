package launcher

import (
	"encoding/json"
	"fmt"
	"strings"
	"wox/common"
	"wox/util"
	"wox/util/osvariant"
)

// UnmarshalJSON routes v2 previews through the same resolver as backend layout; legacy payloads remain unchanged.
func (t *themeData) UnmarshalJSON(data []byte) error {
	var header struct{ SchemaVersion int }
	if err := json.Unmarshal(data, &header); err != nil {
		return err
	}
	if header.SchemaVersion <= 1 {
		type alias themeData
		return json.Unmarshal(data, (*alias)(t))
	}
	var theme common.Theme
	if err := json.Unmarshal(data, &theme); err != nil {
		return err
	}
	resolved, err := theme.ResolveForTarget(util.GetCurrentPlatform(), osvariant.GetCurrentPlatformVariant())
	if err != nil {
		return err
	}
	*t = fromCoreTheme(resolved)
	return nil
}

func isV2Theme(raw map[string]any) bool { return fmt.Sprint(raw["SchemaVersion"]) == "2" }

// themeEditorGroups keeps the legacy editor unchanged and adds required base colors to the v2 window group.
func themeEditorGroups(raw map[string]any) []themeColorGroup {
	if !isV2Theme(raw) {
		return themeEditorColorGroups
	}
	groups := append([]themeColorGroup(nil), themeEditorColorGroups...)
	groups[0].tokens = append([]themeColorToken{
		{key: "BaseBackgroundColor", label: "i18n:ui_theme_base_background"},
		{key: "BaseTextColor", label: "i18n:ui_theme_base_text"},
		{key: "BaseAccentColor", label: "i18n:ui_theme_base_accent"},
	}, groups[0].tokens...)
	groups[2].tokens = append(append([]themeColorToken(nil), groups[2].tokens...), []themeColorToken{{key: "ResultItemHoverBackgroundColor", label: "i18n:ui_theme_editor_token_result_hover_background"}}...)
	groups[4].tokens = append(append([]themeColorToken(nil), groups[4].tokens...), []themeColorToken{{key: "ActionContainerDividerColor", label: "i18n:ui_theme_editor_token_action_divider"}}...)
	groups[5].tokens = append(append([]themeColorToken(nil), groups[5].tokens...), []themeColorToken{{key: "ToolbarHotkeyFontColor", label: "i18n:ui_theme_editor_token_hotkey_text"}, {key: "ToolbarHotkeyBackgroundColor", label: "i18n:ui_theme_editor_token_hotkey_background"}, {key: "ToolbarHotkeyBorderColor", label: "i18n:ui_theme_editor_token_hotkey_border"}}...)
	groups[4].tokens = append(append([]themeColorToken(nil), groups[4].tokens...), []themeColorToken{{key: "ActionItemHotkeyFontColor", label: "i18n:ui_theme_editor_token_action_hotkey_text"}, {key: "ActionItemHotkeyBackgroundColor", label: "i18n:ui_theme_editor_token_action_hotkey_background"}, {key: "ActionItemHotkeyBorderColor", label: "i18n:ui_theme_editor_token_action_hotkey_border"}, {key: "ActionItemActiveHotkeyFontColor", label: "i18n:ui_theme_editor_token_action_active_hotkey_text"}, {key: "ActionItemActiveHotkeyBackgroundColor", label: "i18n:ui_theme_editor_token_action_active_hotkey_background"}, {key: "ActionItemActiveHotkeyBorderColor", label: "i18n:ui_theme_editor_token_action_active_hotkey_border"}}...)

	groups[3].tokens = append(append([]themeColorToken(nil), groups[3].tokens...), []themeColorToken{{key: "PreviewBackgroundColor", label: "i18n:ui_theme_editor_token_preview_background"}, {key: "PreviewBorderColor", label: "i18n:ui_theme_editor_token_preview_border"}, {key: "PreviewTagFontColor", label: "i18n:ui_theme_editor_token_preview_tag_font"}, {key: "PreviewTagBackgroundColor", label: "i18n:ui_theme_editor_token_preview_tag_background"}, {key: "PreviewTagBorderColor", label: "i18n:ui_theme_editor_token_preview_tag_outline"}}...)
	groups[1].tokens = append(append([]themeColorToken(nil), groups[1].tokens...), []themeColorToken{{key: "GlanceFontColor", label: "i18n:ui_theme_editor_token_glance_text"}, {key: "GlanceIconColor", label: "i18n:ui_theme_editor_token_glance_icon"}, {key: "GlanceBackgroundColor", label: "i18n:ui_theme_editor_token_glance_background"}, {key: "GlanceHoverBackgroundColor", label: "i18n:ui_theme_editor_token_glance_hover_background"}}...)
	for i := range groups[3].tokens {
		switch groups[3].tokens[i].key {
		case "PreviewPropertyTitleColor":
			groups[3].tokens[i].label = "i18n:ui_theme_editor_token_preview_property_title"
		case "PreviewPropertyContentColor":
			groups[3].tokens[i].label = "i18n:ui_theme_editor_token_preview_property_value"
		}
	}
	return groups
}

// mergeThemeEditorDraft removes inherited overrides rather than materializing their resolved swatches.
func mergeThemeEditorDraft(raw map[string]any, values map[string]string) map[string]any {
	draft := copyThemeMap(raw)
	for key, value := range values {
		if isV2Theme(raw) {
			value = strings.TrimSpace(value)
		}
		if isV2Theme(raw) && value == "" && key != "ThemeName" && !strings.HasPrefix(key, "Base") {
			delete(draft, key)
		} else {
			draft[key] = value
		}
	}
	return draft
}

// themeEditorResolvedColors supplies effective swatches without changing the authored values.
func themeEditorResolvedColors(raw map[string]any, values map[string]string) map[string]any {
	encoded, _ := json.Marshal(mergeThemeEditorDraft(raw, values))
	var theme common.Theme
	if err := json.Unmarshal(encoded, &theme); err != nil {
		return raw
	}
	resolved, err := theme.ResolveForTarget(util.GetCurrentPlatform(), osvariant.GetCurrentPlatformVariant())
	if err != nil {
		return raw
	}
	type effective common.Theme
	encoded, _ = json.Marshal(effective(resolved))
	var result map[string]any
	_ = json.Unmarshal(encoded, &result)
	for key, value := range resolved.ResolvedColors() {
		result[key] = value
	}
	return result
}

// themeEditorColorValue resolves inheritance for picker display only, never for persistence.
func themeEditorColorValue(raw map[string]any, values map[string]string, key string) string {
	value := values[key]
	if !isV2Theme(raw) {
		return value
	}
	if value == "" {
		value = themeMapString(themeEditorResolvedColors(raw, values), key)
	}
	if c, ok := common.ParseThemeColor(value); ok {
		return fmt.Sprintf("#%02X%02X%02X%02X", c.R, c.G, c.B, c.A)
	}
	return value
}

// resetThemeEditorToken restores a v2 optional color to inheritance.
func (a *App) resetThemeEditorToken() {
	state := a.themeSettings.ThemeEditor()
	if state == nil || !isV2Theme(state.raw) || strings.HasPrefix(state.dialogToken, "Base") {
		return
	}
	index := themeEditorDefinitionIndex(state.definitions, state.dialogToken)
	if index < 0 {
		return
	}
	a.setThemeEditorText(index, "")
	a.confirmThemeEditorDialog()
}
