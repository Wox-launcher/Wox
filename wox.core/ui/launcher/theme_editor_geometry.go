package launcher

import "strings"

// themeEditorTokenSection groups properties by the visible element they style.
func themeEditorTokenSection(key string) string {
	if strings.HasPrefix(key, "Overlay") {
		return ""
	}
	section := "appearance"
	switch {
	case strings.HasPrefix(key, "Glance"):
		section = "glance"
	case strings.HasPrefix(key, "Attention"):
		section = "attention"
	case strings.HasPrefix(key, "Scrollbar"):
		section = "scrollbar"
	case strings.HasPrefix(key, "AppContent"):
		section = "content"
	case strings.HasPrefix(key, "AppBorder"):
		section = "border"
	case strings.HasPrefix(key, "ResultItemActiveIndicator"):
		section = "indicator"
	case strings.HasPrefix(key, "PreviewTag"):
		section = "tags"
	case strings.HasPrefix(key, "PreviewProperty"):
		section = "properties"
	case strings.HasPrefix(key, "ActionQueryBox"):
		section = "query"
	case strings.Contains(key, "Hotkey"):
		section = "hotkeys"
	case strings.HasPrefix(key, "ToolbarPrimary"):
		section = "primary"
	case strings.HasPrefix(key, "ActionItem"), strings.HasPrefix(key, "ResultItemActive"):
		section = "selection"
	case strings.Contains(key, "Padding"):
		section = "spacing"
	}
	return "i18n:ui_theme_editor_section_" + section
}

// Geometry stays in the same surface groups as colors; schema v1 retains its existing editor.
var themeEditorGeometryGroups = [][]themeColorToken{
	{
		{key: "AppBorderWidth", label: "i18n:ui_theme_geometry_AppBorderWidth"},
		{key: "AppBorderRadius", label: "i18n:ui_theme_geometry_AppBorderRadius"},
		{key: "AppContentInset", label: "i18n:ui_theme_geometry_AppContentInset"},
		{key: "AppContentBorderRadius", label: "i18n:ui_theme_geometry_AppContentBorderRadius"},
		{key: "AppPaddingLeft", label: "i18n:ui_theme_geometry_AppPaddingLeft"},
		{key: "AppPaddingTop", label: "i18n:ui_theme_geometry_AppPaddingTop"},
		{key: "AppPaddingRight", label: "i18n:ui_theme_geometry_AppPaddingRight"},
		{key: "AppPaddingBottom", label: "i18n:ui_theme_geometry_AppPaddingBottom"},
		{key: "ScrollbarWidth", label: "i18n:ui_theme_geometry_ScrollbarWidth"},
		{key: "ScrollbarHoverWidth", label: "i18n:ui_theme_geometry_ScrollbarHoverWidth"},
		{key: "ScrollbarBorderRadius", label: "i18n:ui_theme_geometry_ScrollbarBorderRadius"},
	},
	{
		{key: "QueryBoxBorderRadius", label: "i18n:ui_theme_geometry_QueryBoxBorderRadius"},
		{key: "QueryBoxBorderBottomWidth", label: "i18n:ui_theme_geometry_QueryBoxBorderBottomWidth"},
	},
	{
		{key: "ResultContainerPaddingLeft", label: "i18n:ui_theme_geometry_ResultContainerPaddingLeft"},
		{key: "ResultContainerPaddingTop", label: "i18n:ui_theme_geometry_ResultContainerPaddingTop"},
		{key: "ResultContainerPaddingRight", label: "i18n:ui_theme_geometry_ResultContainerPaddingRight"},
		{key: "ResultContainerPaddingBottom", label: "i18n:ui_theme_geometry_ResultContainerPaddingBottom"},
		{key: "ResultItemBorderRadius", label: "i18n:ui_theme_geometry_ResultItemBorderRadius"},
		{key: "ResultItemPaddingLeft", label: "i18n:ui_theme_geometry_ResultItemPaddingLeft"},
		{key: "ResultItemPaddingTop", label: "i18n:ui_theme_geometry_ResultItemPaddingTop"},
		{key: "ResultItemPaddingRight", label: "i18n:ui_theme_geometry_ResultItemPaddingRight"},
		{key: "ResultItemPaddingBottom", label: "i18n:ui_theme_geometry_ResultItemPaddingBottom"},
		{key: "ResultItemActiveIndicatorWidth", label: "i18n:ui_theme_geometry_ResultItemActiveIndicatorWidth"},
		{key: "ResultItemActiveIndicatorInsetLeft", label: "i18n:ui_theme_geometry_ResultItemActiveIndicatorInsetLeft"},
		{key: "ResultItemActiveIndicatorInsetTop", label: "i18n:ui_theme_geometry_ResultItemActiveIndicatorInsetTop"},
		{key: "ResultItemActiveIndicatorInsetBottom", label: "i18n:ui_theme_geometry_ResultItemActiveIndicatorInsetBottom"},
		{key: "ResultItemActiveIndicatorBorderRadius", label: "i18n:ui_theme_geometry_ResultItemActiveIndicatorBorderRadius"},
	},
	{
		{key: "PreviewBorderRadius", label: "i18n:ui_theme_geometry_PreviewBorderRadius"},
		{key: "PreviewTagBorderRadius", label: "i18n:ui_theme_geometry_PreviewTagBorderRadius"},
	},
	{
		{key: "ActionContainerBorderWidth", label: "i18n:ui_theme_geometry_ActionContainerBorderWidth"},
		{key: "ActionContainerBorderRadius", label: "i18n:ui_theme_geometry_ActionContainerBorderRadius"},
		{key: "ActionContainerPaddingLeft", label: "i18n:ui_theme_geometry_ActionContainerPaddingLeft"},
		{key: "ActionContainerPaddingTop", label: "i18n:ui_theme_geometry_ActionContainerPaddingTop"},
		{key: "ActionContainerPaddingRight", label: "i18n:ui_theme_geometry_ActionContainerPaddingRight"},
		{key: "ActionContainerPaddingBottom", label: "i18n:ui_theme_geometry_ActionContainerPaddingBottom"},
		{key: "ActionItemBorderRadius", label: "i18n:ui_theme_geometry_ActionItemBorderRadius"},
		{key: "ActionQueryBoxBorderRadius", label: "i18n:ui_theme_geometry_ActionQueryBoxBorderRadius"},
	},
	{
		{key: "ToolbarBorderWidth", label: "i18n:ui_theme_geometry_ToolbarBorderWidth"},
		{key: "ToolbarPaddingLeft", label: "i18n:ui_theme_geometry_ToolbarPaddingLeft"},
		{key: "ToolbarPaddingRight", label: "i18n:ui_theme_geometry_ToolbarPaddingRight"},
	},
}

// themeEditorNumericToken identifies the optional integer fields accepted by the v2 editor.
func themeEditorNumericToken(key string) bool {
	for _, group := range themeEditorGeometryGroups {
		for _, token := range group {
			if token.key == key {
				return true
			}
		}
	}
	return false
}
