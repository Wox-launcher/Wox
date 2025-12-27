package common

import (
	"encoding/json"
	"strconv"
)

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
}

func (t *Theme) UnmarshalJSON(data []byte) error {
	type themeAlias Theme
	aux := &struct {
		*themeAlias
	}{
		themeAlias: (*themeAlias)(t),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	t.ResultItemBorderLeftWidth = parseJSONInt(raw, "ResultItemBorderLeftWidth", "ResultItemBorderLeft")
	t.ResultItemActiveBorderLeftWidth = parseJSONInt(raw, "ResultItemActiveBorderLeftWidth", "ResultItemActiveBorderLeft")

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
