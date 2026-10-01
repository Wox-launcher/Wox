package component

import (
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
	Theme    ControlTheme
	Active   bool
	// Maximized switches the zoom/maximize control to its restore glyph.
	Maximized  bool
	OnMinimize func()
	OnMaximize func()
	OnClose    func()
}

type windowCloseChromeState struct {
	hovered string
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
	closeID := windowChromeControlID(props.ID, "close")
	minimizeID := windowChromeControlID(props.ID, "minimize")
	maximizeID := windowChromeControlID(props.ID, "maximize")
	children := make([]woxwidget.StackChild, 0, 3)
	switch props.Platform {
	case "darwin":
		// AppKit owns the captions; this layer only retains the title-bar footprint.
	case "linux":
		right := float32(0)
		children = append(children, woxwidget.StackChild{AnchorRight: true, Child: LinuxTitleBarCloseButton(closeID, s.hovered == "close", props.Theme, props.OnClose, onHover)})
		if props.OnMaximize != nil {
			right += TitleBarControlWidth
			icon := MaximizeGlyph(14, TitleBarAlpha(props.Theme.ChromeText, 230))
			if props.Maximized {
				icon = RestoreGlyph(14, TitleBarAlpha(props.Theme.ChromeText, 230))
			}
			children = append(children, woxwidget.StackChild{Right: right, AnchorRight: true, Child: LinuxTitleBarIconButton(
				maximizeID, "maximize", icon, s.hovered == "maximize", false, props.Theme, props.OnMaximize, onHover,
			)})
		}
		if props.OnMinimize != nil {
			right += TitleBarControlWidth
			children = append(children, woxwidget.StackChild{Right: right, AnchorRight: true, Child: LinuxTitleBarIconButton(
				minimizeID, "minimize", MinimizeGlyph(14, TitleBarAlpha(props.Theme.ChromeText, 230)), s.hovered == "minimize", false, props.Theme, props.OnMinimize, onHover,
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
func LinuxTitleBarCloseButton(id string, hovered bool, theme ControlTheme, onTap func(), onHover func(string, bool)) woxwidget.Widget {
	foreground := TitleBarAlpha(theme.ChromeText, 230)
	if hovered {
		foreground = woxui.Color{R: 255, G: 255, B: 255, A: 255}
	}
	return LinuxTitleBarIconButton(id, "close", CloseGlyph(16, foreground), hovered, true, theme, onTap, onHover)
}

// LinuxTitleBarIconButton draws one circular Linux caption control.
func LinuxTitleBarIconButton(id, control string, icon woxwidget.Widget, hovered, danger bool, theme ControlTheme, onTap func(), onHover func(string, bool)) woxwidget.Widget {
	circleColor := woxui.Color{}
	if hovered {
		if danger {
			circleColor = woxui.Color{R: 232, G: 17, B: 35, A: 255}
		} else {
			circleColor = TitleBarAlpha(theme.ChromeText, 26)
		}
	}
	return woxwidget.Gesture{ID: id, OnTap: onTap, OnHover: func(inside bool) {
		if onHover != nil {
			onHover(control, inside)
		}
	}, Child: woxwidget.Container{Width: TitleBarControlWidth, Height: TitleBarHeight, Child: woxwidget.Align{Width: TitleBarControlWidth, Height: TitleBarHeight, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Container{Width: 24, Height: 24, Radius: 12, Color: circleColor, Child: woxwidget.Align{Width: 24, Height: 24, Horizontal: 0.5, Vertical: 0.5, Child: icon}}}}}
}

// WindowsTitleBarButton matches the compact native hover treatment while keeping the frameless window fully custom-drawn.
func WindowsTitleBarButton(id, control string, hovered bool, theme ControlTheme, onTap func(), onHover func(string, bool)) woxwidget.Widget {
	background := woxui.Color{}
	foreground := TitleBarAlpha(theme.ChromeText, 230)
	closeButton := control == "close"
	if hovered {
		background = TitleBarAlpha(theme.ChromeText, 26)
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
