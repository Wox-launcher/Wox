package component

import woxui "wox/ui/runtime"

// Theme contains the semantic appearance shared by Wox launcher components.
type Theme struct {
	PreviewBackgroundColor                     *woxui.Color
	PreviewBorderColor                         *woxui.Color
	PreviewBorderRadius                        *int
	ResultItemActiveIndicatorWidth             *int
	ResultItemActiveIndicatorInsetLeft         *int
	ResultItemActiveIndicatorInsetTop          *int
	ResultItemActiveIndicatorInsetBottom       *int
	ResultItemActiveIndicatorBorderRadius      *int
	QueryBoxBorderBottomWidth                  *int
	ResultItemActiveIndicatorColor             *woxui.Color
	QueryBoxBorderBottomColor                  *woxui.Color
	PreviewTagBorderRadius                     *int
	PreviewTagFontColor                        *woxui.Color
	PreviewTagBackgroundColor                  *woxui.Color
	PreviewTagBorderColor                      *woxui.Color
	GlanceFontColor                            *woxui.Color
	GlanceIconColor                            *woxui.Color
	GlanceBackgroundColor                      *woxui.Color
	RefinementButtonFontColor                  *woxui.Color
	RefinementButtonIconColor                  *woxui.Color
	RefinementButtonBackgroundColor            *woxui.Color
	RefinementButtonBorderColor                *woxui.Color
	RefinementButtonHoverBackgroundColor       *woxui.Color
	RefinementButtonActiveFontColor            *woxui.Color
	RefinementButtonActiveIconColor            *woxui.Color
	RefinementButtonActiveBackgroundColor      *woxui.Color
	RefinementButtonActiveBorderColor          *woxui.Color
	RefinementButtonActiveHoverBackgroundColor *woxui.Color
	RefinementBackgroundColor                  *woxui.Color
	RefinementBorderColor                      *woxui.Color
	RefinementTitleColor                       *woxui.Color
	RefinementDividerColor                     *woxui.Color
	RefinementHotkeyColor                      *woxui.Color
	RefinementItemFontColor                    *woxui.Color
	RefinementItemBackgroundColor              *woxui.Color
	RefinementItemHoverBackgroundColor         *woxui.Color
	RefinementItemActiveFontColor              *woxui.Color
	RefinementItemActiveBackgroundColor        *woxui.Color
	RefinementItemActiveHoverBackgroundColor   *woxui.Color
	ScrollbarThumbColor                        *woxui.Color
	ScrollbarThumbHoverColor                   *woxui.Color
	ScrollbarThumbActiveColor                  *woxui.Color
	ScrollbarWidth                             *int
	ScrollbarHoverWidth                        *int
	ScrollbarBorderRadius                      *int
	AppBorderColor                             *woxui.Color
	AppBorderWidth                             *int
	AppBorderRadius                            *int
	GlanceHoverBackgroundColor                 *woxui.Color

	// Optional v2 colors retain legacy contextual fallbacks when absent.
	ActionContainerDividerColor           *woxui.Color
	ToolbarHotkeyFontColor                *woxui.Color
	ActionItemHotkeyFontColor             *woxui.Color
	ActionItemActiveHotkeyFontColor       *woxui.Color
	ToolbarHotkeyBackgroundColor          *woxui.Color
	ActionItemHotkeyBackgroundColor       *woxui.Color
	ActionItemActiveHotkeyBackgroundColor *woxui.Color
	ToolbarHotkeyBorderColor              *woxui.Color
	ActionItemHotkeyBorderColor           *woxui.Color
	ActionItemActiveHotkeyBorderColor     *woxui.Color
	ResultItemHoverBackgroundColor        *woxui.Color

	// SelectedBorderLeftWidth is launcher result chrome in logical units; zero disables it.
	SelectedBorderLeftWidth float32
	ActionBorderWidth       float32
	ActionContainerRadius   float32
	ActionItemRadius        float32
	ToolbarBorderWidth      float32
	SelectedBorderLeftColor woxui.Color
	ToolbarBorder           woxui.Color

	Background             woxui.Color
	QueryBackground        woxui.Color
	QueryText              woxui.Color
	Cursor                 woxui.Color
	SelectionBackground    woxui.Color
	SelectionText          woxui.Color
	ResultTitle            woxui.Color
	ResultSubtitle         woxui.Color
	ResultTail             woxui.Color
	ErrorText              woxui.Color
	SelectedBackground     woxui.Color
	SelectedTitle          woxui.Color
	SelectedSubtitle       woxui.Color
	SelectedTail           woxui.Color
	ActionBackground       woxui.Color
	ActionBorder           woxui.Color
	ActionHeader           woxui.Color
	ActionText             woxui.Color
	ActionSelected         woxui.Color
	ActionSelectedText     woxui.Color
	PreviewText            woxui.Color
	PreviewSplit           woxui.Color
	PreviewPropertyTitle   woxui.Color
	PreviewPropertyContent woxui.Color
	ToolbarBackground      woxui.Color
	ToolbarText            woxui.Color
}

// DisabledContentAlpha is the shared reduced emphasis for disabled labels and icons.
const DisabledContentAlpha = 88

func withAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}

// ActionDividerColor keeps v1 panel separators coupled to the preview divider.
func (t Theme) ActionDividerColor() woxui.Color {
	if t.ActionContainerDividerColor != nil {
		return *t.ActionContainerDividerColor
	}
	return t.PreviewSplit
}

// ResultHoverColor preserves the legacy selection-derived hover when no override exists.
func (t Theme) ResultHoverColor() woxui.Color {
	if t.ResultItemHoverBackgroundColor != nil {
		return *t.ResultItemHoverBackgroundColor
	}
	c := t.SelectedBackground
	c.A = uint8(float32(c.A)*0.25 + 0.5)
	return c
}

// GlanceColors keeps legacy alpha rules separate from explicit v2 colors.
func (t Theme) GlanceColors(hovered bool) (foreground, background woxui.Color) {
	foreground = t.QueryText
	foreground.A = uint8(float32(foreground.A) * .8)
	if t.GlanceFontColor != nil {
		foreground = *t.GlanceFontColor
	}
	if t.GlanceBackgroundColor != nil {
		background = *t.GlanceBackgroundColor
	}
	if hovered {
		background = t.QueryText
		background.A = uint8(float32(background.A) * .1)
		if t.GlanceHoverBackgroundColor != nil {
			background = *t.GlanceHoverBackgroundColor
		}
	}
	return
}

// GlanceIconTint preserves plugin image handling while allowing an independent glyph tint.
func (t Theme) GlanceIconTint() woxui.Color {
	if t.GlanceIconColor != nil {
		return *t.GlanceIconColor
	}
	c := t.QueryText
	c.A = uint8(float32(c.A) * .8 * .72)
	return c
}

// RefinementButtonColors preserves legacy alpha rules unless a v2 color explicitly overrides them.
func (t Theme) RefinementButtonColors(active, hovered bool) (foreground, icon, background, border woxui.Color) {
	tint := t.QueryText
	backgroundAlpha, borderAlpha, textAlpha := uint8(19), uint8(33), uint8(184)
	textOverride, iconOverride := t.RefinementButtonFontColor, t.RefinementButtonIconColor
	backgroundOverride, borderOverride, hoverOverride := t.RefinementButtonBackgroundColor, t.RefinementButtonBorderColor, t.RefinementButtonHoverBackgroundColor
	if active {
		tint = t.Cursor
		backgroundAlpha, borderAlpha, textAlpha = 38, 82, 240
		textOverride, iconOverride = t.RefinementButtonActiveFontColor, t.RefinementButtonActiveIconColor
		backgroundOverride, borderOverride, hoverOverride = t.RefinementButtonActiveBackgroundColor, t.RefinementButtonActiveBorderColor, t.RefinementButtonActiveHoverBackgroundColor
	}
	foreground, icon, background, border = t.QueryText, tint, tint, tint
	foreground.A, icon.A, background.A, border.A = textAlpha, 235, backgroundAlpha, borderAlpha
	if textOverride != nil {
		foreground = *textOverride
	}
	if iconOverride != nil {
		icon = *iconOverride
	}
	if backgroundOverride != nil {
		background = *backgroundOverride
	}
	if borderOverride != nil {
		border = *borderOverride
	}
	if hovered {
		background = ControlHoverColor(background, tint)
		if hoverOverride != nil {
			background = *hoverOverride
		}
	}
	return
}
