package component

import (
	"math"
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TitleBarHeight is the shared height for custom title bars across windows.
const TitleBarHeight = float32(40)

// TitleBarControlWidth is the pointer target used by Windows and Linux caption buttons.
const TitleBarControlWidth = float32(46)

// TitleBarWindowsIconSize matches the 12-in-40 ratio of native Windows caption glyphs.
const TitleBarWindowsIconSize = float32(12)

// WindowCloseChromeProps describes the shared platform caption controls used by custom title bars.
type WindowCloseChromeProps struct {
	ID       string
	Width    float32
	Platform string
	Theme    Theme
	Active   bool
	// Maximized switches the zoom/maximize control to its restore glyph.
	Maximized  bool
	OnMinimize func()
	OnMaximize func()
	OnClose    func()
}

type windowCloseChromeState struct {
	hovered string
	pressed string
}

// WindowCloseChrome builds the same close control for every custom title bar.
func WindowCloseChrome(props WindowCloseChromeProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: woxwidget.Key(props.ID), Type: (*windowCloseChromeState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &windowCloseChromeState{} },
	}
}

func (s *windowCloseChromeState) InitState(_ woxwidget.StateContext, _ any) {}

func (s *windowCloseChromeState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

// Build retains hover and press state while title-bar content changes.
func (s *windowCloseChromeState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(WindowCloseChromeProps)
	onHover := func(control string, inside bool) {
		context.SetState(func() {
			if inside {
				s.hovered = control
				return
			}
			if s.hovered == control {
				s.hovered = ""
			}
		})
	}
	onPress := func(control string, pressed bool) {
		context.SetState(func() {
			if pressed {
				s.pressed = control
				return
			}
			if s.pressed == control {
				s.pressed = ""
			}
		})
	}
	closeID := windowChromeControlID(props.ID, "close")
	minimizeID := windowChromeControlID(props.ID, "minimize")
	maximizeID := windowChromeControlID(props.ID, "maximize")
	macHovered := s.hovered == "mac-controls"
	children := make([]woxwidget.StackChild, 0, 3)
	switch props.Platform {
	case "darwin":
		children = append(children, woxwidget.StackChild{Left: 13, Child: MacTrafficLight(
			closeID, woxui.Color{R: 255, G: 92, B: 95, A: 255}, "×", woxui.Color{R: 128, G: 47, B: 49, A: 255},
			macHovered, s.pressed == closeID, props.Active, props.Theme, props.OnClose, onHover, onPress,
		)})
		if props.OnMinimize != nil {
			children = append(children, woxwidget.StackChild{Left: 36, Child: MacTrafficLight(
				minimizeID, woxui.Color{R: 250, G: 200, B: 0, A: 255}, "−", woxui.Color{R: 126, G: 100, B: 11, A: 255},
				macHovered, s.pressed == minimizeID, props.Active, props.Theme, props.OnMinimize, onHover, onPress,
			)})
		}
		if props.OnMaximize != nil {
			zoomLeft := float32(36)
			if props.OnMinimize != nil {
				zoomLeft = 59
			}
			children = append(children, woxwidget.StackChild{Left: zoomLeft, Child: MacTrafficLight(
				maximizeID, woxui.Color{R: 40, G: 200, B: 64, A: 255}, "+", woxui.Color{R: 17, G: 96, B: 27, A: 255},
				macHovered, s.pressed == maximizeID, props.Active, props.Theme, props.OnMaximize, onHover, onPress,
			)})
		}
	case "linux":
		right := float32(0)
		children = append(children, woxwidget.StackChild{AnchorRight: true, Child: LinuxTitleBarCloseButton(closeID, s.hovered == "close", props.Theme, props.OnClose, onHover)})
		if props.OnMaximize != nil {
			right += TitleBarControlWidth
			icon := MaximizeGlyph(14, TitleBarAlpha(props.Theme.ToolbarText, 230))
			if props.Maximized {
				icon = RestoreGlyph(14, TitleBarAlpha(props.Theme.ToolbarText, 230))
			}
			children = append(children, woxwidget.StackChild{Right: right, AnchorRight: true, Child: LinuxTitleBarIconButton(
				maximizeID, "maximize", icon, s.hovered == "maximize", false, props.Theme, props.OnMaximize, onHover,
			)})
		}
		if props.OnMinimize != nil {
			right += TitleBarControlWidth
			children = append(children, woxwidget.StackChild{Right: right, AnchorRight: true, Child: LinuxTitleBarIconButton(
				minimizeID, "minimize", MinimizeGlyph(14, TitleBarAlpha(props.Theme.ToolbarText, 230)), s.hovered == "minimize", false, props.Theme, props.OnMinimize, onHover,
			)})
		}
	default:
		right := float32(0)
		children = append(children, woxwidget.StackChild{AnchorRight: true, Child: WindowsTitleBarButton(closeID, "close", s.hovered == "close", props.Theme, props.OnClose, onHover)})
		if props.OnMaximize != nil {
			right += TitleBarControlWidth
			control := "maximize"
			if props.Maximized {
				control = "restore"
			}
			children = append(children, woxwidget.StackChild{Right: right, AnchorRight: true, Child: WindowsTitleBarButton(maximizeID, control, s.hovered == "maximize", props.Theme, props.OnMaximize, onHover)})
		}
		if props.OnMinimize != nil {
			right += TitleBarControlWidth
			children = append(children, woxwidget.StackChild{Right: right, AnchorRight: true, Child: WindowsTitleBarButton(minimizeID, "minimize", s.hovered == "minimize", props.Theme, props.OnMinimize, onHover)})
		}
	}
	return woxwidget.Stack{Width: props.Width, Height: TitleBarHeight, Children: children}
}

// TitleBarChromeWidth returns the trailing space reserved by platform caption buttons.
func TitleBarChromeWidth(platform string, minimize, maximize bool) float32 {
	if platform == "darwin" {
		return 0
	}
	width := TitleBarControlWidth
	if minimize {
		width += TitleBarControlWidth
	}
	if maximize {
		width += TitleBarControlWidth
	}
	return width
}

// windowChromeControlID keeps close IDs stable while adding sibling caption controls.
func windowChromeControlID(id, control string) string {
	if control == "close" {
		return id
	}
	if trimmed, ok := strings.CutSuffix(id, ".close"); ok && trimmed != "" {
		return trimmed + "." + control
	}
	if trimmed, ok := strings.CutSuffix(id, "-close"); ok && trimmed != "" {
		return trimmed + "-" + control
	}
	return id + "." + control
}

func (s *windowCloseChromeState) Dispose() {}

// TitleBarAlpha applies a new alpha channel to a theme color.
func TitleBarAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}

// LinuxTitleBarCloseButton draws the circular Linux close control with a red
// hover fill, matching the compact native treatment.
func LinuxTitleBarCloseButton(id string, hovered bool, theme Theme, onTap func(), onHover func(string, bool)) woxwidget.Widget {
	foreground := TitleBarAlpha(theme.ToolbarText, 230)
	if hovered {
		foreground = woxui.Color{R: 255, G: 255, B: 255, A: 255}
	}
	return LinuxTitleBarIconButton(id, "close", CloseGlyph(16, foreground), hovered, true, theme, onTap, onHover)
}

// LinuxTitleBarIconButton draws one circular Linux caption control.
func LinuxTitleBarIconButton(id, control string, icon woxwidget.Widget, hovered, danger bool, theme Theme, onTap func(), onHover func(string, bool)) woxwidget.Widget {
	circleColor := woxui.Color{}
	if hovered {
		if danger {
			circleColor = woxui.Color{R: 232, G: 17, B: 35, A: 255}
		} else {
			circleColor = TitleBarAlpha(theme.ToolbarText, 26)
		}
	}
	return woxwidget.Gesture{ID: id, OnTap: onTap, OnHover: func(inside bool) {
		if onHover != nil {
			onHover(control, inside)
		}
	}, Child: woxwidget.Container{Width: TitleBarControlWidth, Height: TitleBarHeight, Child: woxwidget.Align{Width: TitleBarControlWidth, Height: TitleBarHeight, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Container{Width: 24, Height: 24, Radius: 12, Color: circleColor, Child: woxwidget.Align{Width: 24, Height: 24, Horizontal: 0.5, Vertical: 0.5, Child: icon}}}}}
}

// WindowsTitleBarButton matches the compact native hover treatment while keeping the frameless window fully custom-drawn.
func WindowsTitleBarButton(id, control string, hovered bool, theme Theme, onTap func(), onHover func(string, bool)) woxwidget.Widget {
	background := woxui.Color{}
	foreground := TitleBarAlpha(theme.ToolbarText, 230)
	closeButton := control == "close"
	if hovered {
		background = TitleBarAlpha(theme.ToolbarText, 26)
		if closeButton {
			background = woxui.Color{R: 232, G: 17, B: 35, A: 255}
			foreground = woxui.Color{R: 255, G: 255, B: 255, A: 255}
		}
	}
	hoverName := windowsTitleBarControlName(id, closeButton)
	return woxwidget.Gesture{ID: id, OnTap: onTap, OnHover: func(inside bool) {
		if onHover != nil {
			onHover(hoverName, inside)
		}
	}, Child: woxwidget.Container{Width: TitleBarControlWidth, Height: TitleBarHeight, Color: background, Child: woxwidget.Align{Width: TitleBarControlWidth, Height: TitleBarHeight, Horizontal: 0.5, Vertical: 0.5, Child: windowsTitleBarGlyph(control, foreground)}}}
}

// windowsTitleBarGlyph draws the Segoe-style caption mark for one Windows chrome button.
func windowsTitleBarGlyph(control string, color woxui.Color) woxwidget.Widget {
	switch control {
	case "close":
		return CloseGlyph(TitleBarWindowsIconSize, color)
	case "minimize":
		return MinimizeGlyph(TitleBarWindowsIconSize, color)
	case "restore":
		return RestoreGlyph(TitleBarWindowsIconSize, color)
	default:
		return MaximizeGlyph(TitleBarWindowsIconSize, color)
	}
}

// windowsTitleBarControlName maps a caption button to its hover-group name.
func windowsTitleBarControlName(id string, closeButton bool) string {
	if closeButton {
		return "close"
	}
	if strings.Contains(id, "maximize") {
		return "maximize"
	}
	return "minimize"
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
			symbol = MacZoomGlyph(glyphColor)
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

// MacZoomGlyph draws a geometrically centered plus. A font "+" sits on the text
// baseline and looks shifted up and right inside the 14-unit traffic light.
func MacZoomGlyph(color woxui.Color) woxwidget.Widget {
	return woxwidget.Painter{Width: 14, Height: 14, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		displayList.FillRoundedRect(woxui.Rect{X: bounds.X + 4, Y: bounds.Y + 6, Width: 6, Height: 2}, 1, color)
		displayList.FillRoundedRect(woxui.Rect{X: bounds.X + 6, Y: bounds.Y + 4, Width: 2, Height: 6}, 1, color)
	}}
}
