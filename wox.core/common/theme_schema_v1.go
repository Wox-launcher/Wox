package common

import (
	"encoding/json"
	"strconv"
)

// ThemeSchemaV1 preserves the legacy field types and zero-value semantics.
type ThemeSchemaV1 struct {
	SchemaVersion int
	MinWoxVersion string `json:",omitempty"`

	ThemeId     string
	ThemeName   string
	ThemeAuthor string
	ThemeUrl    string
	Version     string
	Description string
	IsSystem    bool
	IsInstalled bool

	IsAutoAppearance bool   // Whether to automatically switch theme based on system appearance
	DarkThemeId      string // ID of the dark theme variant
	LightThemeId     string // ID of the light theme variant

	AppBackgroundColor                   string
	AppPaddingLeft                       int
	AppPaddingTop                        int
	AppPaddingRight                      int
	AppPaddingBottom                     int
	ResultContainerPaddingLeft           int
	ResultContainerPaddingTop            int
	ResultContainerPaddingRight          int
	ResultContainerPaddingBottom         int
	ResultItemBorderRadius               int
	ResultItemPaddingLeft                int
	ResultItemPaddingTop                 int
	ResultItemPaddingRight               int
	ResultItemPaddingBottom              int
	ResultItemTitleColor                 string
	ResultItemSubTitleColor              string
	ResultItemTailTextColor              string
	ResultItemBorderLeftWidth            int
	ResultItemActiveBackgroundColor      string
	ResultItemActiveTitleColor           string
	ResultItemActiveSubTitleColor        string
	ResultItemActiveBorderLeftWidth      int
	ResultItemActiveBorderLeftColor      string `json:",omitempty"`
	ResultItemActiveTailTextColor        string
	QueryBoxFontColor                    string
	QueryBoxBackgroundColor              string
	QueryBoxBorderRadius                 int
	QueryBoxCursorColor                  string
	QueryBoxTextSelectionBackgroundColor string
	QueryBoxTextSelectionColor           string
	ActionContainerBackgroundColor       string
	ActionContainerBorderColor           string `json:",omitempty"`
	ActionContainerBorderWidth           *int   `json:",omitempty"` // Nil preserves the legacy one-unit border; zero disables it.
	ActionContainerHeaderFontColor       string
	ActionContainerBorderRadius          *int `json:",omitempty"`
	ActionItemBorderRadius               *int `json:",omitempty"`
	ActionContainerPaddingLeft           int
	ActionContainerPaddingTop            int
	ActionContainerPaddingRight          int
	ActionContainerPaddingBottom         int
	ActionItemActiveBackgroundColor      string
	ActionItemActiveFontColor            string
	ActionItemFontColor                  string
	ActionQueryBoxFontColor              string
	ActionQueryBoxBackgroundColor        string
	ActionQueryBoxBorderRadius           int
	PreviewFontColor                     string
	PreviewSplitLineColor                string
	PreviewPropertyTitleColor            string
	PreviewPropertyContentColor          string
	PreviewTextSelectionColor            string
	ToolbarFontColor                     string
	ToolbarBackgroundColor               string
	ToolbarBorderColor                   string `json:",omitempty"`
	ToolbarBorderWidth                   *int   `json:",omitempty"`
	ToolbarPaddingLeft                   int
	ToolbarPaddingRight                  int

	Windows *ThemePlatformOverride `json:"windows,omitempty"`
	MacOS   *ThemePlatformOverride `json:"macos,omitempty"`
	Linux   *ThemePlatformOverride `json:"linux,omitempty"`
}

func init() {
	registerThemeSchema(1, themeSchema{parse: parseThemeV1, marshal: marshalThemeV1, resolve: resolveThemeV1})
}

// parseThemeV1 accepts historical aliases without applying v2 defaults.
func parseThemeV1(data []byte) (Theme, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Theme{}, err
	}
	for _, platform := range []string{"windows", "macos", "linux"} {
		if err := validateThemePlatformOverride(raw, platform, themeV1StyleFields); err != nil {
			return Theme{}, err
		}
	}
	var legacy ThemeSchemaV1
	if err := json.Unmarshal(data, &legacy); err != nil {
		return Theme{}, err
	}
	legacy.SchemaVersion = 1
	legacy.ResultItemBorderLeftWidth = parseJSONInt(raw, "ResultItemBorderLeftWidth", "ResultItemBorderLeft")
	legacy.ResultItemActiveBorderLeftWidth = parseJSONInt(raw, "ResultItemActiveBorderLeftWidth", "ResultItemActiveBorderLeft")
	return Theme{
		SchemaVersion:                        legacy.SchemaVersion,
		MinWoxVersion:                        legacy.MinWoxVersion,
		ThemeId:                              legacy.ThemeId,
		ThemeName:                            legacy.ThemeName,
		ThemeAuthor:                          legacy.ThemeAuthor,
		ThemeUrl:                             legacy.ThemeUrl,
		Version:                              legacy.Version,
		Description:                          legacy.Description,
		IsSystem:                             legacy.IsSystem,
		IsInstalled:                          legacy.IsInstalled,
		IsAutoAppearance:                     legacy.IsAutoAppearance,
		DarkThemeId:                          legacy.DarkThemeId,
		LightThemeId:                         legacy.LightThemeId,
		AppBackgroundColor:                   legacy.AppBackgroundColor,
		AppPaddingLeft:                       legacy.AppPaddingLeft,
		AppPaddingTop:                        legacy.AppPaddingTop,
		AppPaddingRight:                      legacy.AppPaddingRight,
		AppPaddingBottom:                     legacy.AppPaddingBottom,
		ResultContainerPaddingLeft:           legacy.ResultContainerPaddingLeft,
		ResultContainerPaddingTop:            legacy.ResultContainerPaddingTop,
		ResultContainerPaddingRight:          legacy.ResultContainerPaddingRight,
		ResultContainerPaddingBottom:         legacy.ResultContainerPaddingBottom,
		ResultItemBorderRadius:               legacy.ResultItemBorderRadius,
		ResultItemPaddingLeft:                legacy.ResultItemPaddingLeft,
		ResultItemPaddingTop:                 legacy.ResultItemPaddingTop,
		ResultItemPaddingRight:               legacy.ResultItemPaddingRight,
		ResultItemPaddingBottom:              legacy.ResultItemPaddingBottom,
		ResultItemTitleColor:                 legacy.ResultItemTitleColor,
		ResultItemSubTitleColor:              legacy.ResultItemSubTitleColor,
		ResultItemTailTextColor:              legacy.ResultItemTailTextColor,
		ResultItemBorderLeftWidth:            legacy.ResultItemBorderLeftWidth,
		ResultItemActiveBackgroundColor:      legacy.ResultItemActiveBackgroundColor,
		ResultItemActiveTitleColor:           legacy.ResultItemActiveTitleColor,
		ResultItemActiveSubTitleColor:        legacy.ResultItemActiveSubTitleColor,
		ResultItemActiveBorderLeftWidth:      legacy.ResultItemActiveBorderLeftWidth,
		ResultItemActiveBorderLeftColor:      legacy.ResultItemActiveBorderLeftColor,
		ResultItemActiveTailTextColor:        legacy.ResultItemActiveTailTextColor,
		QueryBoxFontColor:                    legacy.QueryBoxFontColor,
		QueryBoxBackgroundColor:              legacy.QueryBoxBackgroundColor,
		QueryBoxBorderRadius:                 legacy.QueryBoxBorderRadius,
		QueryBoxCursorColor:                  legacy.QueryBoxCursorColor,
		QueryBoxTextSelectionBackgroundColor: legacy.QueryBoxTextSelectionBackgroundColor,
		QueryBoxTextSelectionColor:           legacy.QueryBoxTextSelectionColor,
		ActionContainerBackgroundColor:       legacy.ActionContainerBackgroundColor,
		ActionContainerBorderColor:           legacy.ActionContainerBorderColor,
		ActionContainerBorderWidth:           legacy.ActionContainerBorderWidth,
		ActionContainerHeaderFontColor:       legacy.ActionContainerHeaderFontColor,
		ActionContainerBorderRadius:          legacy.ActionContainerBorderRadius,
		ActionItemBorderRadius:               legacy.ActionItemBorderRadius,
		ActionContainerPaddingLeft:           legacy.ActionContainerPaddingLeft,
		ActionContainerPaddingTop:            legacy.ActionContainerPaddingTop,
		ActionContainerPaddingRight:          legacy.ActionContainerPaddingRight,
		ActionContainerPaddingBottom:         legacy.ActionContainerPaddingBottom,
		ActionItemActiveBackgroundColor:      legacy.ActionItemActiveBackgroundColor,
		ActionItemActiveFontColor:            legacy.ActionItemActiveFontColor,
		ActionItemFontColor:                  legacy.ActionItemFontColor,
		ActionQueryBoxFontColor:              legacy.ActionQueryBoxFontColor,
		ActionQueryBoxBackgroundColor:        legacy.ActionQueryBoxBackgroundColor,
		ActionQueryBoxBorderRadius:           legacy.ActionQueryBoxBorderRadius,
		PreviewFontColor:                     legacy.PreviewFontColor,
		PreviewSplitLineColor:                legacy.PreviewSplitLineColor,
		PreviewPropertyTitleColor:            legacy.PreviewPropertyTitleColor,
		PreviewPropertyContentColor:          legacy.PreviewPropertyContentColor,
		PreviewTextSelectionColor:            legacy.PreviewTextSelectionColor,
		ToolbarFontColor:                     legacy.ToolbarFontColor,
		ToolbarBackgroundColor:               legacy.ToolbarBackgroundColor,
		ToolbarBorderColor:                   legacy.ToolbarBorderColor,
		ToolbarBorderWidth:                   legacy.ToolbarBorderWidth,
		ToolbarPaddingLeft:                   legacy.ToolbarPaddingLeft,
		ToolbarPaddingRight:                  legacy.ToolbarPaddingRight,
		Windows:                              legacy.Windows,
		MacOS:                                legacy.MacOS,
		Linux:                                legacy.Linux,
	}, nil
}

func marshalThemeV1(t Theme) ([]byte, error) {
	t.SchemaVersion = 1
	return json.Marshal(ThemeSchemaV1{
		SchemaVersion:                        t.SchemaVersion,
		MinWoxVersion:                        t.MinWoxVersion,
		ThemeId:                              t.ThemeId,
		ThemeName:                            t.ThemeName,
		ThemeAuthor:                          t.ThemeAuthor,
		ThemeUrl:                             t.ThemeUrl,
		Version:                              t.Version,
		Description:                          t.Description,
		IsSystem:                             t.IsSystem,
		IsInstalled:                          t.IsInstalled,
		IsAutoAppearance:                     t.IsAutoAppearance,
		DarkThemeId:                          t.DarkThemeId,
		LightThemeId:                         t.LightThemeId,
		AppBackgroundColor:                   t.AppBackgroundColor,
		AppPaddingLeft:                       t.AppPaddingLeft,
		AppPaddingTop:                        t.AppPaddingTop,
		AppPaddingRight:                      t.AppPaddingRight,
		AppPaddingBottom:                     t.AppPaddingBottom,
		ResultContainerPaddingLeft:           t.ResultContainerPaddingLeft,
		ResultContainerPaddingTop:            t.ResultContainerPaddingTop,
		ResultContainerPaddingRight:          t.ResultContainerPaddingRight,
		ResultContainerPaddingBottom:         t.ResultContainerPaddingBottom,
		ResultItemBorderRadius:               t.ResultItemBorderRadius,
		ResultItemPaddingLeft:                t.ResultItemPaddingLeft,
		ResultItemPaddingTop:                 t.ResultItemPaddingTop,
		ResultItemPaddingRight:               t.ResultItemPaddingRight,
		ResultItemPaddingBottom:              t.ResultItemPaddingBottom,
		ResultItemTitleColor:                 t.ResultItemTitleColor,
		ResultItemSubTitleColor:              t.ResultItemSubTitleColor,
		ResultItemTailTextColor:              t.ResultItemTailTextColor,
		ResultItemBorderLeftWidth:            t.ResultItemBorderLeftWidth,
		ResultItemActiveBackgroundColor:      t.ResultItemActiveBackgroundColor,
		ResultItemActiveTitleColor:           t.ResultItemActiveTitleColor,
		ResultItemActiveSubTitleColor:        t.ResultItemActiveSubTitleColor,
		ResultItemActiveBorderLeftWidth:      t.ResultItemActiveBorderLeftWidth,
		ResultItemActiveBorderLeftColor:      t.ResultItemActiveBorderLeftColor,
		ResultItemActiveTailTextColor:        t.ResultItemActiveTailTextColor,
		QueryBoxFontColor:                    t.QueryBoxFontColor,
		QueryBoxBackgroundColor:              t.QueryBoxBackgroundColor,
		QueryBoxBorderRadius:                 t.QueryBoxBorderRadius,
		QueryBoxCursorColor:                  t.QueryBoxCursorColor,
		QueryBoxTextSelectionBackgroundColor: t.QueryBoxTextSelectionBackgroundColor,
		QueryBoxTextSelectionColor:           t.QueryBoxTextSelectionColor,
		ActionContainerBackgroundColor:       t.ActionContainerBackgroundColor,
		ActionContainerBorderColor:           t.ActionContainerBorderColor,
		ActionContainerBorderWidth:           t.ActionContainerBorderWidth,
		ActionContainerHeaderFontColor:       t.ActionContainerHeaderFontColor,
		ActionContainerBorderRadius:          t.ActionContainerBorderRadius,
		ActionItemBorderRadius:               t.ActionItemBorderRadius,
		ActionContainerPaddingLeft:           t.ActionContainerPaddingLeft,
		ActionContainerPaddingTop:            t.ActionContainerPaddingTop,
		ActionContainerPaddingRight:          t.ActionContainerPaddingRight,
		ActionContainerPaddingBottom:         t.ActionContainerPaddingBottom,
		ActionItemActiveBackgroundColor:      t.ActionItemActiveBackgroundColor,
		ActionItemActiveFontColor:            t.ActionItemActiveFontColor,
		ActionItemFontColor:                  t.ActionItemFontColor,
		ActionQueryBoxFontColor:              t.ActionQueryBoxFontColor,
		ActionQueryBoxBackgroundColor:        t.ActionQueryBoxBackgroundColor,
		ActionQueryBoxBorderRadius:           t.ActionQueryBoxBorderRadius,
		PreviewFontColor:                     t.PreviewFontColor,
		PreviewSplitLineColor:                t.PreviewSplitLineColor,
		PreviewPropertyTitleColor:            t.PreviewPropertyTitleColor,
		PreviewPropertyContentColor:          t.PreviewPropertyContentColor,
		PreviewTextSelectionColor:            t.PreviewTextSelectionColor,
		ToolbarFontColor:                     t.ToolbarFontColor,
		ToolbarBackgroundColor:               t.ToolbarBackgroundColor,
		ToolbarBorderColor:                   t.ToolbarBorderColor,
		ToolbarBorderWidth:                   t.ToolbarBorderWidth,
		ToolbarPaddingLeft:                   t.ToolbarPaddingLeft,
		ToolbarPaddingRight:                  t.ToolbarPaddingRight,
		Windows:                              t.Windows,
		MacOS:                                t.MacOS,
		Linux:                                t.Linux,
	})
}

// Only visual fields can be overridden by a platform node. Identity and control
// fields stay top-level so a theme cannot become a different theme only on one
// OS, and invalid keys fail while parsing instead of being silently ignored.
var themeV1StyleFields = map[string]bool{
	"BaseBackgroundColor":                  true,
	"BaseTextColor":                        true,
	"BaseAccentColor":                      true,
	"AppBackgroundColor":                   true,
	"AppPaddingLeft":                       true,
	"AppPaddingTop":                        true,
	"AppPaddingRight":                      true,
	"AppPaddingBottom":                     true,
	"ResultContainerPaddingLeft":           true,
	"ResultContainerPaddingTop":            true,
	"ResultContainerPaddingRight":          true,
	"ResultContainerPaddingBottom":         true,
	"ResultItemBorderRadius":               true,
	"ResultItemPaddingLeft":                true,
	"ResultItemPaddingTop":                 true,
	"ResultItemPaddingRight":               true,
	"ResultItemPaddingBottom":              true,
	"ResultItemTitleColor":                 true,
	"ResultItemSubTitleColor":              true,
	"ResultItemTailTextColor":              true,
	"ResultItemBorderLeftWidth":            true,
	"ResultItemBorderLeft":                 true,
	"ResultItemActiveBackgroundColor":      true,
	"ResultItemActiveTitleColor":           true,
	"ResultItemActiveSubTitleColor":        true,
	"ResultItemActiveBorderLeftWidth":      true,
	"ResultItemActiveBorderLeftColor":      true,
	"ResultItemActiveBorderLeft":           true,
	"ResultItemActiveTailTextColor":        true,
	"QueryBoxFontColor":                    true,
	"QueryBoxBackgroundColor":              true,
	"QueryBoxBorderRadius":                 true,
	"QueryBoxCursorColor":                  true,
	"QueryBoxTextSelectionBackgroundColor": true,
	"QueryBoxTextSelectionColor":           true,
	"ActionContainerBackgroundColor":       true,
	"ActionContainerBorderColor":           true,
	"ActionContainerBorderWidth":           true,
	"ActionContainerBorderRadius":          true,
	"ActionItemBorderRadius":               true,
	"ActionContainerHeaderFontColor":       true,
	"ActionContainerPaddingLeft":           true,
	"ActionContainerPaddingTop":            true,
	"ActionContainerPaddingRight":          true,
	"ActionContainerPaddingBottom":         true,
	"ActionItemActiveBackgroundColor":      true,
	"ActionItemActiveFontColor":            true,
	"ActionItemFontColor":                  true,
	"ActionQueryBoxFontColor":              true,
	"ActionQueryBoxBackgroundColor":        true,
	"ActionQueryBoxBorderRadius":           true,
	"PreviewFontColor":                     true,
	"PreviewSplitLineColor":                true,
	"PreviewPropertyTitleColor":            true,
	"PreviewPropertyContentColor":          true,
	"PreviewTextSelectionColor":            true,
	"ToolbarFontColor":                     true,
	"ToolbarBackgroundColor":               true,
	"ToolbarBorderColor":                   true,
	"ToolbarBorderWidth":                   true,
	"ToolbarPaddingLeft":                   true,
	"ToolbarPaddingRight":                  true,
}

func parseJSONInt(raw map[string]json.RawMessage, keys ...string) int {
	for _, key := range keys {
		value, ok := raw[key]
		if !ok || len(value) == 0 {
			continue
		}
		if string(value) == "null" {
			continue
		}

		var intValue int
		if err := json.Unmarshal(value, &intValue); err == nil {
			return intValue
		}

		var floatValue float64
		if err := json.Unmarshal(value, &floatValue); err == nil {
			return int(floatValue)
		}

		var strValue string
		if err := json.Unmarshal(value, &strValue); err == nil {
			if parsed, parseErr := strconv.Atoi(strValue); parseErr == nil {
				return parsed
			}
		}
	}

	return 0
}

// resolveThemeV1 keeps legacy alias precedence and null/zero behavior when flattening platform styles.
func resolveThemeV1(t Theme, platform, variant string) (Theme, error) {
	var node *ThemePlatformOverride
	switch platform {
	case "windows":
		node = t.Windows
	case "darwin", "macos":
		node = t.MacOS
	case "linux":
		node = t.Linux
	}
	t.Windows, t.MacOS, t.Linux = nil, nil, nil
	if node == nil || len(*node) == 0 {
		return t, nil
	}
	encoded, err := json.Marshal(t)
	if err != nil {
		return Theme{}, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return Theme{}, err
	}
	merge := func(fields ThemePlatformOverride) {
		// An authored legacy alias must win over the canonical base field.
		if _, ok := fields["ResultItemBorderLeft"]; ok {
			delete(raw, "ResultItemBorderLeftWidth")
		}
		if _, ok := fields["ResultItemActiveBorderLeft"]; ok {
			delete(raw, "ResultItemActiveBorderLeftWidth")
		}
		for key, value := range fields {
			if key != "variants" {
				raw[key] = value
			}
		}
	}
	merge(*node)
	if data := (*node)["variants"]; variant != "" && len(data) > 0 {
		var variants map[string]ThemePlatformOverride
		if err := json.Unmarshal(data, &variants); err != nil {
			return Theme{}, err
		}
		merge(variants[variant])
	}
	encoded, err = json.Marshal(raw)
	if err != nil {
		return Theme{}, err
	}
	return parseThemeV1(encoded)
}
