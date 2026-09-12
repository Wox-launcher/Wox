package common

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"reflect"
	"strconv"
	"strings"
)

// ThemeSchemaV2 is the complete v2 document. Nil styles inherit; zero and transparent are explicit overrides.
type ThemeSchemaV2 struct {
	SchemaVersion    int
	MinWoxVersion    string `json:",omitempty"`
	ThemeId          string
	ThemeName        string
	ThemeAuthor      string `json:",omitempty"`
	ThemeUrl         string `json:",omitempty"`
	Version          string `json:",omitempty"`
	Description      string `json:",omitempty"`
	IsSystem         bool   `json:",omitempty"`
	IsInstalled      bool   `json:",omitempty"`
	IsAutoAppearance bool   `json:",omitempty"`
	DarkThemeId      string `json:",omitempty"`
	LightThemeId     string `json:",omitempty"`

	BaseBackgroundColor string
	BaseTextColor       string
	BaseAccentColor     string

	// Any authored AppBorder* field disables system window material on every
	// platform (Acrylic, Liquid Glass, compositor blur) so Go UI can paint the outline.
	AppBorderColor  *string `json:",omitempty"`
	AppBorderWidth  *int    `json:",omitempty"`
	AppBorderRadius *int    `json:",omitempty"`

	AppBackgroundColor *string `json:",omitempty"`
	AppPaddingLeft     *int    `json:",omitempty"`
	AppPaddingTop      *int    `json:",omitempty"`
	AppPaddingRight    *int    `json:",omitempty"`
	AppPaddingBottom   *int    `json:",omitempty"`

	QueryBoxFontColor                    *string `json:",omitempty"`
	QueryBoxBackgroundColor              *string `json:",omitempty"`
	QueryBoxBorderRadius                 *int    `json:",omitempty"`
	QueryBoxCursorColor                  *string `json:",omitempty"`
	QueryBoxBorderBottomColor            *string `json:",omitempty"`
	QueryBoxBorderBottomWidth            *int    `json:",omitempty"`
	QueryBoxTextSelectionBackgroundColor *string `json:",omitempty"`
	QueryBoxTextSelectionColor           *string `json:",omitempty"`

	ScrollbarThumbColor       *string `json:",omitempty"`
	ScrollbarThumbHoverColor  *string `json:",omitempty"`
	ScrollbarThumbActiveColor *string `json:",omitempty"`
	ScrollbarWidth            *int    `json:",omitempty"`
	ScrollbarHoverWidth       *int    `json:",omitempty"`
	ScrollbarBorderRadius     *int    `json:",omitempty"`

	RefinementButtonFontColor                  *string `json:",omitempty"`
	RefinementButtonIconColor                  *string `json:",omitempty"`
	RefinementButtonBackgroundColor            *string `json:",omitempty"`
	RefinementButtonBorderColor                *string `json:",omitempty"`
	RefinementButtonHoverBackgroundColor       *string `json:",omitempty"`
	RefinementButtonActiveFontColor            *string `json:",omitempty"`
	RefinementButtonActiveIconColor            *string `json:",omitempty"`
	RefinementButtonActiveBackgroundColor      *string `json:",omitempty"`
	RefinementButtonActiveBorderColor          *string `json:",omitempty"`
	RefinementButtonActiveHoverBackgroundColor *string `json:",omitempty"`

	RefinementBackgroundColor                *string `json:",omitempty"`
	RefinementBorderColor                    *string `json:",omitempty"`
	RefinementTitleColor                     *string `json:",omitempty"`
	RefinementDividerColor                   *string `json:",omitempty"`
	RefinementHotkeyColor                    *string `json:",omitempty"`
	RefinementItemFontColor                  *string `json:",omitempty"`
	RefinementItemBackgroundColor            *string `json:",omitempty"`
	RefinementItemHoverBackgroundColor       *string `json:",omitempty"`
	RefinementItemActiveFontColor            *string `json:",omitempty"`
	RefinementItemActiveBackgroundColor      *string `json:",omitempty"`
	RefinementItemActiveHoverBackgroundColor *string `json:",omitempty"`

	GlanceFontColor            *string `json:",omitempty"`
	GlanceIconColor            *string `json:",omitempty"`
	GlanceBackgroundColor      *string `json:",omitempty"`
	GlanceHoverBackgroundColor *string `json:",omitempty"`

	ResultContainerPaddingLeft   *int `json:",omitempty"`
	ResultContainerPaddingTop    *int `json:",omitempty"`
	ResultContainerPaddingRight  *int `json:",omitempty"`
	ResultContainerPaddingBottom *int `json:",omitempty"`

	ResultItemBorderRadius                *int    `json:",omitempty"`
	ResultItemPaddingLeft                 *int    `json:",omitempty"`
	ResultItemPaddingTop                  *int    `json:",omitempty"`
	ResultItemPaddingRight                *int    `json:",omitempty"`
	ResultItemPaddingBottom               *int    `json:",omitempty"`
	ResultItemTitleColor                  *string `json:",omitempty"`
	ResultItemSubTitleColor               *string `json:",omitempty"`
	ResultItemTailTextColor               *string `json:",omitempty"`
	ResultItemBorderLeftWidth             *int    `json:",omitempty"`
	ResultItemHoverBackgroundColor        *string `json:",omitempty"`
	ResultItemActiveBackgroundColor       *string `json:",omitempty"`
	ResultItemActiveTitleColor            *string `json:",omitempty"`
	ResultItemActiveSubTitleColor         *string `json:",omitempty"`
	ResultItemActiveIndicatorWidth        *int    `json:",omitempty"`
	ResultItemActiveIndicatorInsetLeft    *int    `json:",omitempty"`
	ResultItemActiveIndicatorInsetTop     *int    `json:",omitempty"`
	ResultItemActiveIndicatorInsetBottom  *int    `json:",omitempty"`
	ResultItemActiveIndicatorBorderRadius *int    `json:",omitempty"`
	ResultItemActiveIndicatorColor        *string `json:",omitempty"`
	ResultItemActiveTailTextColor         *string `json:",omitempty"`

	ActionContainerDividerColor    *string `json:",omitempty"`
	ActionContainerBackgroundColor *string `json:",omitempty"`
	ActionContainerBorderColor     *string `json:",omitempty"`
	ActionContainerBorderWidth     *int    `json:",omitempty"`
	ActionContainerHeaderFontColor *string `json:",omitempty"`
	ActionContainerBorderRadius    *int    `json:",omitempty"`
	ActionContainerPaddingLeft     *int    `json:",omitempty"`
	ActionContainerPaddingTop      *int    `json:",omitempty"`
	ActionContainerPaddingRight    *int    `json:",omitempty"`
	ActionContainerPaddingBottom   *int    `json:",omitempty"`

	ActionItemBorderRadius          *int    `json:",omitempty"`
	ActionItemActiveBackgroundColor *string `json:",omitempty"`
	ActionItemActiveFontColor       *string `json:",omitempty"`
	ActionItemFontColor             *string `json:",omitempty"`

	ActionQueryBoxFontColor       *string `json:",omitempty"`
	ActionQueryBoxBackgroundColor *string `json:",omitempty"`
	ActionQueryBoxBorderRadius    *int    `json:",omitempty"`

	PreviewBackgroundColor      *string `json:",omitempty"`
	PreviewBorderColor          *string `json:",omitempty"`
	PreviewBorderRadius         *int    `json:",omitempty"`
	PreviewTagBorderRadius      *int    `json:",omitempty"`
	PreviewTagFontColor         *string `json:",omitempty"`
	PreviewTagBackgroundColor   *string `json:",omitempty"`
	PreviewTagBorderColor       *string `json:",omitempty"`
	PreviewFontColor            *string `json:",omitempty"`
	PreviewSplitLineColor       *string `json:",omitempty"`
	PreviewPropertyTitleColor   *string `json:",omitempty"`
	PreviewPropertyContentColor *string `json:",omitempty"`
	PreviewTextSelectionColor   *string `json:",omitempty"`

	ToolbarHotkeyFontColor       *string `json:",omitempty"`
	ToolbarHotkeyBackgroundColor *string `json:",omitempty"`
	ToolbarHotkeyBorderColor     *string `json:",omitempty"`

	ActionItemHotkeyFontColor       *string `json:",omitempty"`
	ActionItemHotkeyBackgroundColor *string `json:",omitempty"`
	ActionItemHotkeyBorderColor     *string `json:",omitempty"`

	ActionItemActiveHotkeyFontColor       *string `json:",omitempty"`
	ActionItemActiveHotkeyBackgroundColor *string `json:",omitempty"`
	ActionItemActiveHotkeyBorderColor     *string `json:",omitempty"`

	ToolbarFontColor       *string `json:",omitempty"`
	ToolbarBackgroundColor *string `json:",omitempty"`
	ToolbarBorderColor     *string `json:",omitempty"`
	ToolbarBorderWidth     *int    `json:",omitempty"`
	ToolbarPaddingLeft     *int    `json:",omitempty"`
	ToolbarPaddingRight    *int    `json:",omitempty"`

	Windows *ThemePlatformOverride `json:"windows,omitempty"`
	MacOS   *ThemePlatformOverride `json:"macos,omitempty"`
	Linux   *ThemePlatformOverride `json:"linux,omitempty"`
}

func init() {
	registerThemeSchema(2, themeSchema{parse: parseThemeV2, marshal: marshalThemeV2, resolve: resolveThemeV2, colors: themeV2Colors})
}

// parseThemeV2 resolves optional styles while retaining their authored presence.
func parseThemeV2(data []byte) (Theme, error) {
	var t Theme
	err := decodeThemeV2(data, &t)
	return t, err
}

func decodeThemeV2(data []byte, t *Theme) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	for _, platform := range []string{"windows", "macos", "linux"} {
		if err := validateThemePlatformOverride(raw, platform, themeV2StyleFields); err != nil {
			return err
		}
	}
	for key := range raw {
		if !themeV2DocumentFields[key] {
			return fmt.Errorf("unknown v2 theme field %q", key)
		}
	}
	var definition ThemeSchemaV2
	if err := json.Unmarshal(data, &definition); err != nil {
		return err
	}
	if definition.ThemeId == "" || definition.ThemeName == "" {
		return fmt.Errorf("v2 theme requires ThemeId and ThemeName")
	}
	*t = Theme{
		SchemaVersion:    definition.SchemaVersion,
		MinWoxVersion:    definition.MinWoxVersion,
		ThemeId:          definition.ThemeId,
		ThemeName:        definition.ThemeName,
		ThemeAuthor:      definition.ThemeAuthor,
		ThemeUrl:         definition.ThemeUrl,
		Version:          definition.Version,
		Description:      definition.Description,
		IsSystem:         definition.IsSystem,
		IsInstalled:      definition.IsInstalled,
		IsAutoAppearance: definition.IsAutoAppearance,
		DarkThemeId:      definition.DarkThemeId,
		LightThemeId:     definition.LightThemeId,
		Windows:          definition.Windows, MacOS: definition.MacOS, Linux: definition.Linux,
	}
	type themeAlias Theme
	resolved, err := definition.resolve()
	if err != nil {
		return err
	}
	if err := json.Unmarshal(resolved, (*themeAlias)(t)); err != nil {
		return err
	}
	var resolvedFields map[string]json.RawMessage
	if err := json.Unmarshal(resolved, &resolvedFields); err != nil {
		return err
	}
	colors := make(map[string]string)
	for key, value := range resolvedFields {
		if strings.HasSuffix(key, "Color") {
			var css string
			if err := json.Unmarshal(value, &css); err != nil {
				return err
			}
			colors[key] = css
		}
	}
	t.source = &themeV2Source{
		definition:      definition,
		colors:          colors,
		appWindowChrome: authoredAppWindowChrome(definition.AppBorderColor, definition.AppBorderWidth, definition.AppBorderRadius),
	}
	if err := validateV2ThemePlatforms(raw); err != nil {
		return err
	}
	return nil
}

// resolveThemeV2 merges authored platform overrides before deriving any default colors or geometry.
func resolveThemeV2(t Theme, platform, variant string) (Theme, error) {
	if platform == "darwin" {
		platform = "macos"
	}
	encoded, err := json.Marshal(t)
	if err != nil {
		return Theme{}, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &raw); err != nil {
		return Theme{}, err
	}
	var node map[string]json.RawMessage
	if data := raw[platform]; len(data) > 0 {
		if err := json.Unmarshal(data, &node); err != nil {
			return Theme{}, err
		}
	}
	merge := func(fields map[string]json.RawMessage) {
		for key, value := range fields {
			if key != "variants" && string(value) != "null" {
				raw[key] = value
			}
		}
	}
	merge(node)
	if data := node["variants"]; len(data) > 0 {
		var variants map[string]map[string]json.RawMessage
		if err := json.Unmarshal(data, &variants); err != nil {
			return Theme{}, err
		}
		merge(variants[variant])
	}
	delete(raw, "windows")
	delete(raw, "macos")
	delete(raw, "linux")
	encoded, err = json.Marshal(raw)
	if err != nil {
		return Theme{}, err
	}
	var resolved Theme
	if err := json.Unmarshal(encoded, &resolved); err != nil {
		return Theme{}, err
	}
	// Keep the original authored source alongside effective fields for lossless editing and sync.
	// Window chrome follows the merged document so a platform-only outline still disables material.
	resolved.source.(*themeV2Source).definition = t.source.(*themeV2Source).definition
	resolved.source.(*themeV2Source).appWindowChrome = rawHasAppWindowChrome(raw)
	resolved.Windows, resolved.MacOS, resolved.Linux = t.Windows, t.MacOS, t.Linux
	return resolved, nil
}

func authoredAppWindowChrome(color *string, width *int, radius *int) bool {
	return color != nil || width != nil || radius != nil
}

func rawHasAppWindowChrome(raw map[string]json.RawMessage) bool {
	for _, key := range []string{"AppBorderColor", "AppBorderWidth", "AppBorderRadius"} {
		if value, ok := raw[key]; ok && string(value) != "null" {
			return true
		}
	}
	return false
}

// marshalThemeV2 preserves authored v2 overrides instead of exporting resolved defaults.
func marshalThemeV2(t Theme) ([]byte, error) {
	source, ok := t.source.(*themeV2Source)
	if !ok || source == nil {
		return nil, fmt.Errorf("v2 theme has no authored definition")
	}
	definition := source.definition
	// Metadata can change on install or save-as; styles stay exactly as authored.
	definition.SchemaVersion = t.SchemaVersion
	definition.MinWoxVersion = t.MinWoxVersion
	definition.ThemeId = t.ThemeId
	definition.ThemeName = t.ThemeName
	definition.ThemeAuthor = t.ThemeAuthor
	definition.ThemeUrl = t.ThemeUrl
	definition.Version = t.Version
	definition.Description = t.Description
	definition.IsSystem = t.IsSystem
	definition.IsInstalled = t.IsInstalled
	definition.IsAutoAppearance = t.IsAutoAppearance
	definition.DarkThemeId = t.DarkThemeId
	definition.LightThemeId = t.LightThemeId
	definition.Windows, definition.MacOS, definition.Linux = t.Windows, t.MacOS, t.Linux
	if _, err := definition.resolve(); err != nil {
		return nil, err
	}
	return json.Marshal(definition)
}

// validateV2ThemePlatforms checks every authored variant, including platforms other than the running OS.
func validateV2ThemePlatforms(raw map[string]json.RawMessage) error {
	for _, platform := range []string{"windows", "macos", "linux"} {
		if len(raw[platform]) == 0 || string(raw[platform]) == "null" {
			continue
		}
		var node map[string]json.RawMessage
		if err := json.Unmarshal(raw[platform], &node); err != nil {
			return err
		}
		variants := map[string]map[string]json.RawMessage{"": nil}
		if data := node["variants"]; len(data) > 0 {
			if err := json.Unmarshal(data, &variants); err != nil {
				return err
			}
			variants[""] = nil
		}
		for variant, fields := range variants {
			merged := make(map[string]json.RawMessage, len(raw))
			for key, value := range raw {
				if key != "windows" && key != "macos" && key != "linux" {
					merged[key] = value
				}
			}
			for _, overrides := range []map[string]json.RawMessage{node, fields} {
				for key, value := range overrides {
					if key != "variants" && string(value) != "null" {
						merged[key] = value
					}
				}
			}
			data, err := json.Marshal(merged)
			if err != nil {
				return err
			}
			var theme Theme
			if err := json.Unmarshal(data, &theme); err != nil {
				return fmt.Errorf("%s/%s: %w", platform, variant, err)
			}
		}
	}
	return nil
}

// resolve applies the versioned v2 defaults once, before backend geometry or UI adapters consume the theme.
func (d ThemeSchemaV2) resolve() ([]byte, error) {
	bases := []struct{ name, value string }{{"BaseBackgroundColor", d.BaseBackgroundColor}, {"BaseTextColor", d.BaseTextColor}, {"BaseAccentColor", d.BaseAccentColor}}
	for _, base := range bases {
		if _, ok := ParseThemeColor(base.value); !ok {
			return nil, fmt.Errorf("%s must be a valid color", base.name)
		}
	}
	text, _ := ParseThemeColor(d.BaseTextColor)
	accent, _ := ParseThemeColor(d.BaseAccentColor)
	secondary := themeColorOpacity(text, .72)
	divider := themeColorOpacity(text, .16)
	selected := themeColorOpacity(accent, .18)
	selection := themeColorOpacity(accent, .35)
	values := map[string]any{
		"AppBorderColor":      divider,
		"ScrollbarThumbColor": themeColorOpacity(text, .58), "ScrollbarThumbHoverColor": themeColorOpacity(text, .72), "ScrollbarThumbActiveColor": themeColorOpacity(accent, .85),
		"RefinementBackgroundColor":                themeColorOpacity(text, .035),
		"RefinementBorderColor":                    themeColorOpacity(text, .12),
		"RefinementTitleColor":                     themeColorOpacity(text, .68),
		"RefinementDividerColor":                   themeColorOpacity(text, .13),
		"RefinementHotkeyColor":                    themeColorOpacity(text, .58),
		"RefinementItemFontColor":                  themeColorOpacity(text, .82),
		"RefinementItemBackgroundColor":            "transparent",
		"RefinementItemHoverBackgroundColor":       themeColorOpacity(text, .08),
		"RefinementItemActiveFontColor":            d.BaseTextColor,
		"RefinementItemActiveBackgroundColor":      selected,
		"RefinementItemActiveHoverBackgroundColor": themeColorOpacity(accent, .26),

		"RefinementButtonFontColor":                  secondary,
		"RefinementButtonIconColor":                  themeColorOpacity(text, .92),
		"RefinementButtonBackgroundColor":            themeColorOpacity(text, .075),
		"RefinementButtonBorderColor":                themeColorOpacity(text, .13),
		"RefinementButtonHoverBackgroundColor":       themeColorOpacity(text, .14),
		"RefinementButtonActiveFontColor":            themeColorOpacity(text, .94),
		"RefinementButtonActiveIconColor":            themeColorOpacity(accent, .92),
		"RefinementButtonActiveBackgroundColor":      themeColorOpacity(accent, .15),
		"RefinementButtonActiveBorderColor":          themeColorOpacity(accent, .32),
		"RefinementButtonActiveHoverBackgroundColor": themeColorOpacity(accent, .22),

		"PreviewBackgroundColor": d.BaseBackgroundColor, "PreviewBorderColor": divider, "PreviewTagFontColor": secondary, "PreviewTagBackgroundColor": "transparent", "PreviewTagBorderColor": divider, "GlanceFontColor": secondary, "GlanceIconColor": secondary, "GlanceBackgroundColor": "transparent", "GlanceHoverBackgroundColor": themeColorOpacity(text, .1),
		"ActionContainerDividerColor": divider, "ActionItemHotkeyFontColor": secondary, "ActionItemHotkeyBackgroundColor": "transparent", "ActionItemHotkeyBorderColor": divider, "ActionItemActiveHotkeyFontColor": d.BaseTextColor, "ActionItemActiveHotkeyBackgroundColor": "transparent", "ActionItemActiveHotkeyBorderColor": d.BaseTextColor, "ToolbarHotkeyFontColor": secondary, "ToolbarHotkeyBackgroundColor": "transparent", "ToolbarHotkeyBorderColor": divider, "ResultItemHoverBackgroundColor": themeColorOpacity(accent, 0.045),
		"AppBackgroundColor": d.BaseBackgroundColor,
		"AppPaddingLeft":     10, "AppPaddingTop": 10, "AppPaddingRight": 10, "AppPaddingBottom": 10,
		"ResultContainerPaddingLeft": 0, "ResultContainerPaddingTop": 8, "ResultContainerPaddingRight": 0, "ResultContainerPaddingBottom": 0,
		"ResultItemBorderRadius": 8, "ResultItemPaddingLeft": 8, "ResultItemPaddingTop": 3, "ResultItemPaddingRight": 8, "ResultItemPaddingBottom": 3,
		"ResultItemTitleColor": d.BaseTextColor, "ResultItemSubTitleColor": secondary, "ResultItemTailTextColor": secondary,
		"QueryBoxBorderBottomColor": d.BaseAccentColor, "QueryBoxBorderBottomWidth": 0, "ResultItemBorderLeftWidth": 0, "ResultItemActiveIndicatorWidth": 0, "ResultItemActiveIndicatorColor": d.BaseAccentColor,
		"ResultItemActiveBackgroundColor": selected, "ResultItemActiveTitleColor": d.BaseTextColor, "ResultItemActiveSubTitleColor": d.BaseTextColor, "ResultItemActiveTailTextColor": d.BaseTextColor,
		"QueryBoxFontColor": d.BaseTextColor, "QueryBoxBackgroundColor": d.BaseBackgroundColor, "QueryBoxBorderRadius": 8, "QueryBoxCursorColor": d.BaseAccentColor,
		"QueryBoxTextSelectionBackgroundColor": selection, "QueryBoxTextSelectionColor": d.BaseTextColor,
		"ActionContainerBackgroundColor": d.BaseBackgroundColor, "ActionContainerBorderColor": divider, "ActionContainerBorderWidth": 1, "ActionContainerBorderRadius": 8,
		"ActionContainerHeaderFontColor": secondary, "ActionContainerPaddingLeft": 14, "ActionContainerPaddingTop": 10, "ActionContainerPaddingRight": 14, "ActionContainerPaddingBottom": 10,
		"ActionItemBorderRadius": 8, "ActionItemActiveBackgroundColor": selected, "ActionItemActiveFontColor": d.BaseTextColor, "ActionItemFontColor": d.BaseTextColor,
		"ActionQueryBoxFontColor": d.BaseTextColor, "ActionQueryBoxBackgroundColor": d.BaseBackgroundColor, "ActionQueryBoxBorderRadius": 8,
		"PreviewFontColor": d.BaseTextColor, "PreviewSplitLineColor": divider, "PreviewPropertyTitleColor": secondary, "PreviewPropertyContentColor": d.BaseTextColor, "PreviewTextSelectionColor": selection,
		"ToolbarFontColor": secondary, "ToolbarBackgroundColor": d.BaseBackgroundColor, "ToolbarBorderColor": divider, "ToolbarBorderWidth": 1, "ToolbarPaddingLeft": 10, "ToolbarPaddingRight": 10,
	}
	encoded, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	var overrides map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &overrides); err != nil {
		return nil, err
	}
	for key, value := range overrides {
		if !themeV2StyleFields[key] || strings.HasPrefix(key, "Base") {
			continue
		}
		if strings.HasSuffix(key, "Color") {
			var css string
			if err := json.Unmarshal(value, &css); err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			parsed, ok := ParseThemeColor(css)
			if !ok {
				return nil, fmt.Errorf("%s must be a valid color", key)
			}
			// Normalize v2 colors so every existing renderer accepts exactly the validated value.
			values[key] = themeColorOpacity(parsed, 1)
		} else {
			var number int
			if err := json.Unmarshal(value, &number); err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			if number < 0 {
				return nil, fmt.Errorf("%s must be non-negative", key)
			}
			values[key] = number
		}
	}
	// Base values also pass through normalization (notably the CSS transparent keyword).
	for key, value := range values {
		if css, ok := value.(string); ok {
			parsed, _ := ParseThemeColor(css)
			values[key] = themeColorOpacity(parsed, 1)
		}
	}
	return json.Marshal(values)
}

func themeColorOpacity(c color.NRGBA, opacity float64) string {
	return fmt.Sprintf("#%02X%02X%02X%02X", c.R, c.G, c.B, uint8(math.Floor(float64(c.A)*opacity)))
}

// ParseThemeColor validates v2 CSS colors strictly; legacy UI parsing remains unchanged.
func ParseThemeColor(value string) (color.NRGBA, bool) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "transparent" {
		return color.NRGBA{}, true
	}
	if strings.HasPrefix(value, "#") && (len(value) == 7 || len(value) == 9) {
		data, err := hex.DecodeString(value[1:])
		if err != nil {
			return color.NRGBA{}, false
		}
		c := color.NRGBA{R: data[0], G: data[1], B: data[2], A: 255}
		if len(data) == 4 {
			c.A = data[3]
		}
		return c, true
	}
	count := 3
	prefix := "rgb("
	if strings.HasPrefix(value, "rgba(") {
		count, prefix = 4, "rgba("
	}
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, ")") {
		return color.NRGBA{}, false
	}
	parts := strings.Split(value[len(prefix):len(value)-1], ",")
	if len(parts) != count {
		return color.NRGBA{}, false
	}
	channels := [4]uint8{0, 0, 0, 255}
	for i, part := range parts {
		n, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		limit := float64(255)
		if i == 3 {
			limit = 1
		}
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < 0 || n > limit {
			return color.NRGBA{}, false
		}
		if i == 3 {
			n *= 255
		}
		channels[i] = uint8(math.Floor(n))
	}
	return color.NRGBA{R: channels[0], G: channels[1], B: channels[2], A: channels[3]}, true
}

// themeV2Source keeps authored styles separate from target-specific resolved colors.
type themeV2Source struct {
	definition      ThemeSchemaV2
	colors          map[string]string
	appWindowChrome bool
}

func themeV2Colors(t Theme) map[string]string {
	if source, ok := t.source.(*themeV2Source); ok {
		return source.colors
	}
	return nil
}

// The complete v2 declaration is the source of truth for accepted JSON fields.
var themeV2DocumentFields = func() map[string]bool {
	fields := make(map[string]bool)
	typ := reflect.TypeFor[ThemeSchemaV2]()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" {
			name = field.Name
		}
		fields[name] = true
	}
	return fields
}()

// Only v2 style fields may occur in platform overrides; identity stays at the root.
var themeV2StyleFields = func() map[string]bool {
	fields := make(map[string]bool, len(themeV2DocumentFields))
	for key := range themeV2DocumentFields {
		fields[key] = true
	}
	for _, key := range []string{"SchemaVersion", "MinWoxVersion", "ThemeId", "ThemeName", "ThemeAuthor", "ThemeUrl", "Version", "Description", "IsSystem", "IsInstalled", "IsAutoAppearance", "DarkThemeId", "LightThemeId", "windows", "macos", "linux"} {
		delete(fields, key)
	}
	return fields
}()
