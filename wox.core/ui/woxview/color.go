package woxview

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
)

// Color holds either one literal/token or a complete light/dark pair. The value
// stays immutable in the tree; the embedding surface chooses its branch per build.
type Color struct {
	Value ColorValue
	Light ColorValue
	Dark  ColorValue
}

// ColorValue is one literal or semantic token; dual-color branches cannot nest.
type ColorValue string

// UnmarshalJSON preserves scalar colors and rejects incomplete or nested pairs.
func (c *Color) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if bytes.Equal(data, []byte("null")) {
		*c = Color{}
		return nil
	}
	var value ColorValue
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*c = Color{Value: value}
		return c.validate()
	}
	var pair struct {
		Light ColorValue `json:"light"`
		Dark  ColorValue `json:"dark"`
	}
	if err := decodeStrict(data, &pair); err != nil {
		return errors.New("color must be a string or an object with light and dark colors")
	}
	if pair.Light == "" || pair.Dark == "" {
		return errors.New("color requires nonempty light and dark values")
	}
	*c = Color{Light: pair.Light, Dark: pair.Dark}
	return c.validate()
}

// MarshalJSON keeps the wire contract scalar unless a dual color was authored.
func (c Color) MarshalJSON() ([]byte, error) {
	if err := c.validate(); err != nil {
		return nil, err
	}
	if c.Light == "" && c.Dark == "" {
		return json.Marshal(c.Value)
	}
	return json.Marshal(struct {
		Light ColorValue `json:"light"`
		Dark  ColorValue `json:"dark"`
	}{c.Light, c.Dark})
}

// validate checks both branches even when only one is currently visible.
func (c Color) validate() error {
	if c.Light == "" && c.Dark == "" {
		return c.Value.validate()
	}
	if c.Value != "" || c.Light == "" || c.Dark == "" {
		return errors.New("color must contain either a value or both light and dark colors")
	}
	if err := c.Light.validate(); err != nil {
		return fmt.Errorf("light: %w", err)
	}
	if err := c.Dark.validate(); err != nil {
		return fmt.Errorf("dark: %w", err)
	}
	return nil
}

// resolve uses the current surface appearance without mutating retained props.
func (c Color) resolve(theme component.ControlTheme, fallback ui.Color) ui.Color {
	value := c.Value
	if c.Light != "" || c.Dark != "" {
		value = c.Light
		if theme.Dark {
			value = c.Dark
		}
	}
	return value.resolve(theme, fallback)
}

// ColorToken names a theme color resolved by the embedding feature's ControlTheme.
const (
	ColorText          ColorValue = "$text"
	ColorTextSecondary ColorValue = "$textSecondary"
	ColorAccent        ColorValue = "$accent"
	ColorAccentText    ColorValue = "$accentText"
	ColorBackground    ColorValue = "$background"
	ColorSurface       ColorValue = "$surface"
	ColorBorder        ColorValue = "$border"
	ColorSuccess       ColorValue = "$success"
	ColorWarning       ColorValue = "$warning"
	ColorError         ColorValue = "$error"
	ColorHover         ColorValue = "$hover"
	ColorSelected      ColorValue = "$selected"
	ColorSelectedText  ColorValue = "$selectedText"
	ColorFocus         ColorValue = "$focus"
	ColorDisabled      ColorValue = "$disabled"
	ColorLink          ColorValue = "$link"
	ColorOverlay       ColorValue = "$overlay"
	ColorWarningText   ColorValue = "$warningText"
)

func (c ColorValue) validate() error {
	if c == "" {
		return nil
	}
	if strings.HasPrefix(string(c), "$") {
		if _, ok := themeColor(component.ControlTheme{}, c); !ok {
			return fmt.Errorf("unknown color token %q", c)
		}
		return nil
	}
	if _, ok := parseHexColor(string(c)); !ok {
		return fmt.Errorf("invalid color %q", c)
	}
	return nil
}

// resolve returns the concrete color, or fallback when the color is unset.
func (c ColorValue) resolve(theme component.ControlTheme, fallback ui.Color) ui.Color {
	if c == "" {
		return fallback
	}
	if value, ok := themeColor(theme, c); ok {
		return value
	}
	if value, ok := parseHexColor(string(c)); ok {
		return value
	}
	return fallback
}

func themeColor(theme component.ControlTheme, token ColorValue) (ui.Color, bool) {
	switch token {
	case ColorText:
		return theme.Text, true
	case ColorTextSecondary:
		return theme.TextSecondary, true
	case ColorAccent:
		return theme.Accent, true
	case ColorAccentText:
		return theme.AccentText, true
	case ColorBackground:
		return theme.Background, true
	case ColorSurface:
		return theme.Surface, true
	case ColorBorder:
		return theme.Border, true
	case ColorSuccess:
		return theme.Success, true
	case ColorWarning:
		return theme.Warning, true
	case ColorError:
		return theme.Error, true
	case ColorHover:
		return theme.Hover, true
	case ColorSelected:
		return theme.SelectionBackground, true
	case ColorSelectedText:
		return theme.SelectionText, true
	case ColorFocus:
		return theme.Focus, true
	case ColorDisabled:
		return theme.Disabled, true
	case ColorLink:
		return theme.Link, true
	case ColorOverlay:
		return theme.Overlay, true
	case ColorWarningText:
		return theme.WarningText, true
	}
	return ui.Color{}, false
}

func parseHexColor(value string) (ui.Color, bool) {
	hex, ok := strings.CutPrefix(value, "#")
	if !ok {
		return ui.Color{}, false
	}
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) == 6 {
		hex += "ff"
	}
	if len(hex) != 8 {
		return ui.Color{}, false
	}
	parsed, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return ui.Color{}, false
	}
	return ui.Color{R: uint8(parsed >> 24), G: uint8(parsed >> 16), B: uint8(parsed >> 8), A: uint8(parsed)}, true
}
