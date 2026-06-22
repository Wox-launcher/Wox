package common

import (
	"encoding/json"
	"fmt"
	"strconv"
)

const themePlatformOverrideVariantsField = "variants"

// ThemePlatformOverride preserves a raw top-level platform node from theme JSON.
// The backend merges the node for the current OS before sending the flat theme to
// Flutter, while keeping the raw node available so store-installed themes do not
// lose platform-specific settings for other operating systems.
type ThemePlatformOverride map[string]json.RawMessage

type Theme struct {
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
	ResultItemActiveTailTextColor        string
	QueryBoxFontColor                    string
	QueryBoxBackgroundColor              string
	QueryBoxBorderRadius                 int
	QueryBoxCursorColor                  string
	QueryBoxTextSelectionBackgroundColor string
	QueryBoxTextSelectionColor           string
	ActionContainerBackgroundColor       string
	ActionContainerHeaderFontColor       string
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
	ToolbarPaddingLeft                   int
	ToolbarPaddingRight                  int

	Windows *ThemePlatformOverride `json:"windows,omitempty"`
	MacOS   *ThemePlatformOverride `json:"macos,omitempty"`
	Linux   *ThemePlatformOverride `json:"linux,omitempty"`
}

func (t *Theme) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	for _, platformName := range []string{"windows", "macos", "linux"} {
		if err := validateThemePlatformOverride(raw, platformName); err != nil {
			return err
		}
	}

	type themeAlias Theme
	aux := &struct {
		*themeAlias
	}{
		themeAlias: (*themeAlias)(t),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	t.ResultItemBorderLeftWidth = parseJSONInt(raw, "ResultItemBorderLeftWidth", "ResultItemBorderLeft")
	t.ResultItemActiveBorderLeftWidth = parseJSONInt(raw, "ResultItemActiveBorderLeftWidth", "ResultItemActiveBorderLeft")

	return nil
}

// Only visual fields can be overridden by a platform node. Identity and control
// fields stay top-level so a theme cannot become a different theme only on one
// OS, and invalid keys fail while parsing instead of being silently ignored.
var themePlatformOverrideStyleFields = map[string]bool{
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
	"ResultItemActiveBorderLeft":           true,
	"ResultItemActiveTailTextColor":        true,
	"QueryBoxFontColor":                    true,
	"QueryBoxBackgroundColor":              true,
	"QueryBoxBorderRadius":                 true,
	"QueryBoxCursorColor":                  true,
	"QueryBoxTextSelectionBackgroundColor": true,
	"QueryBoxTextSelectionColor":           true,
	"ActionContainerBackgroundColor":       true,
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
	"ToolbarPaddingLeft":                   true,
	"ToolbarPaddingRight":                  true,
}

func validateThemePlatformOverride(raw map[string]json.RawMessage, platformName string) error {
	value, ok := raw[platformName]
	if !ok {
		return nil
	}
	if string(value) == "null" {
		return nil
	}

	var overrides map[string]json.RawMessage
	if err := json.Unmarshal(value, &overrides); err != nil {
		return fmt.Errorf("platform theme override %q must be a JSON object: %w", platformName, err)
	}

	for fieldName := range overrides {
		if fieldName == themePlatformOverrideVariantsField {
			if err := validateThemePlatformOverrideVariants(overrides[fieldName], platformName); err != nil {
				return err
			}
			continue
		}
		if !themePlatformOverrideStyleFields[fieldName] {
			return fmt.Errorf("platform theme override %q contains non-style field %q", platformName, fieldName)
		}
	}

	return nil
}

func validateThemePlatformOverrideVariants(value json.RawMessage, platformName string) error {
	if string(value) == "null" {
		return fmt.Errorf("platform theme override %q variants must be a JSON object", platformName)
	}

	var variants map[string]json.RawMessage
	if err := json.Unmarshal(value, &variants); err != nil {
		return fmt.Errorf("platform theme override %q variants must be a JSON object: %w", platformName, err)
	}
	if variants == nil {
		return fmt.Errorf("platform theme override %q variants must be a JSON object", platformName)
	}

	for variantName, variantValue := range variants {
		if string(variantValue) == "null" {
			return fmt.Errorf("platform theme override %q variant %q must be a JSON object", platformName, variantName)
		}

		var overrides map[string]json.RawMessage
		if err := json.Unmarshal(variantValue, &overrides); err != nil {
			return fmt.Errorf("platform theme override %q variant %q must be a JSON object: %w", platformName, variantName, err)
		}
		if overrides == nil {
			return fmt.Errorf("platform theme override %q variant %q must be a JSON object", platformName, variantName)
		}

		for fieldName := range overrides {
			if !themePlatformOverrideStyleFields[fieldName] {
				return fmt.Errorf("platform theme override %q variant %q contains non-style field %q", platformName, variantName, fieldName)
			}
		}
	}

	return nil
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
