package component

import (
	"math"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TitleBarHeight is the shared height for custom title bars across windows.
const TitleBarHeight = float32(40)

// TitleBarAlpha applies a new alpha channel to a theme color.
func TitleBarAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}

// LinuxTitleBarCloseButton draws the circular Linux close control with a red
// hover fill, matching the compact native treatment.
func LinuxTitleBarCloseButton(id string, hovered bool, theme Theme, onTap func(), onHover func(string, bool)) woxwidget.Widget {
	foreground := TitleBarAlpha(theme.ToolbarText, 230)
	circleColor := woxui.Color{}
	if hovered {
		circleColor = woxui.Color{R: 232, G: 17, B: 35, A: 255}
		foreground = woxui.Color{R: 255, G: 255, B: 255, A: 255}
	}
	return woxwidget.Gesture{ID: id, OnTap: onTap, OnHover: func(inside bool) {
		if onHover != nil {
			onHover("close", inside)
		}
	}, Child: woxwidget.Container{Width: 46, Height: TitleBarHeight, Child: woxwidget.Align{Width: 46, Height: TitleBarHeight, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Container{Width: 24, Height: 24, Radius: 12, Color: circleColor, Child: woxwidget.Align{Width: 24, Height: 24, Horizontal: 0.5, Vertical: 0.5, Child: CloseGlyph(16, foreground)}}}}}
}

// WindowsTitleBarButton matches the compact native hover treatment while keeping the frameless window fully custom-drawn.
func WindowsTitleBarButton(id, glyph string, closeButton, hovered bool, theme Theme, onTap func(), onHover func(string, bool)) woxwidget.Widget {
	background := woxui.Color{}
	foreground := TitleBarAlpha(theme.ToolbarText, 230)
	if hovered {
		background = TitleBarAlpha(theme.ToolbarText, 26)
		if closeButton {
			background = woxui.Color{R: 232, G: 17, B: 35, A: 255}
			foreground = woxui.Color{R: 255, G: 255, B: 255, A: 255}
		}
	}
	control := "minimize"
	if closeButton {
		control = "close"
	}
	return woxwidget.Gesture{ID: id, OnTap: onTap, OnHover: func(inside bool) {
		if onHover != nil {
			onHover(control, inside)
		}
	}, Child: woxwidget.Container{Width: 46, Height: TitleBarHeight, Color: background, Child: woxwidget.Align{Width: 46, Height: TitleBarHeight, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: glyph, Style: woxui.TextStyle{Size: 18}, Color: foreground}}}}
}

// MacTrafficLight matches the compact macOS controls and reveals their glyphs while the group is hovered.
// Inactive (non-key) windows use a uniform gray fill until the group is hovered, matching AppKit.
func MacTrafficLight(id string, color woxui.Color, glyph string, glyphColor woxui.Color, hovered, pressed, active bool, theme Theme, onTap func(), onHover, onPress func(string, bool)) woxwidget.Widget {
	if !active && !hovered {
		color = MacTrafficLightInactiveColor(theme)
	}
	if pressed {
		color = MacTrafficLightPressedColor(color)
	}
	var symbol woxwidget.Widget = woxwidget.Container{Width: 14, Height: 14}
	if hovered {
		switch glyph {
		case "×":
			symbol = MacCloseGlyph(glyphColor)
		case "−":
			symbol = woxwidget.Container{Width: 7, Height: 2, Radius: 1, Color: glyphColor}
		default:
			symbol = woxwidget.Text{Value: glyph, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: glyphColor}
		}
	}
	control := woxwidget.Align{Width: 20, Height: TitleBarHeight, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Container{Width: 14, Height: 14, Radius: 7, Color: color, Child: woxwidget.Align{Width: 14, Height: 14, Horizontal: 0.5, Vertical: 0.5, Child: symbol}}}
	if onTap == nil && onHover == nil && onPress == nil {
		return control
	}
	return woxwidget.Gesture{ID: id, OnTap: onTap, OnPressChange: func(pressed bool) {
		if onPress != nil {
			onPress(id, pressed)
		}
	}, OnHover: func(inside bool) {
		if onHover != nil {
			onHover("mac-controls", inside)
		}
	}, Child: control}
}

// MacTrafficLightInactiveColor is the unfocused fill used by native macOS traffic lights.
func MacTrafficLightInactiveColor(theme Theme) woxui.Color {
	if macTrafficLightThemeIsDark(theme) {
		return woxui.Color{R: 94, G: 94, B: 96, A: 255}
	}
	return woxui.Color{R: 222, G: 222, B: 222, A: 255}
}

// macTrafficLightThemeIsDark uses relative luminance so inactive gray tracks light and dark title bars.
func macTrafficLightThemeIsDark(theme Theme) bool {
	linear := func(value uint8) float64 {
		channel := float64(value) / 255
		if channel <= 0.03928 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(theme.Background.R)+0.7152*linear(theme.Background.G)+0.0722*linear(theme.Background.B) < 0.5
}

// MacTrafficLightPressedColor approximates AppKit's highlighted luminance while preserving hue.
func MacTrafficLightPressedColor(color woxui.Color) woxui.Color {
	color.R = uint8(uint16(color.R) * 220 / 255)
	color.G = uint8(uint16(color.G) * 220 / 255)
	color.B = uint8(uint16(color.B) * 220 / 255)
	return color
}

// MacCloseGlyph draws the thicker cross used by the native macOS traffic light.
func MacCloseGlyph(color woxui.Color) woxwidget.Widget {
	return woxwidget.Painter{Width: 14, Height: 14, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		for step := 0; step < 5; step++ {
			offset := float32(step)
			displayList.FillRoundedRect(woxui.Rect{X: bounds.X + 4 + offset, Y: bounds.Y + 4 + offset, Width: 2, Height: 2}, 1, color)
			displayList.FillRoundedRect(woxui.Rect{X: bounds.X + 8 - offset, Y: bounds.Y + 4 + offset, Width: 2, Height: 2}, 1, color)
		}
	}}
}
