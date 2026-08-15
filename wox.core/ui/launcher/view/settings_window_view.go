package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// SettingsWindowProps contains the prepared rail, page, and optional modal overlay.
type SettingsWindowProps struct {
	Width       float32
	Height      float32
	Radius      float32
	PageID      string
	Platform    string
	RailWidth   float32
	TitleBar    woxwidget.Widget
	Rail        woxwidget.Widget
	Page        woxwidget.Widget
	Overlay     woxwidget.Widget
	OverlayLeft float32
	OverlayTop  float32
	Theme       woxcomponent.Theme
}

const SettingsTitleBarHeight = woxcomponent.TitleBarHeight

// SettingsWindow builds the shared settings window frame.
func SettingsWindow(props SettingsWindowProps) woxwidget.Widget {
	contentHeight := max(float32(0), props.Height-SettingsTitleBarHeight)
	page := woxwidget.Semantics{
		Key: "settings-page-key", AutomationID: "settings.page." + props.PageID, Role: woxui.AccessibilityRoleGroup, Label: props.PageID + " settings",
		Child: props.Page,
	}
	var bodyChild woxwidget.Widget
	if props.Platform == "darwin" {
		// macOS window controls belong to the rail, so the page should not reserve the rail's title-bar height.
		bodyChild = woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
			{Left: props.RailWidth, Child: page},
			{Top: SettingsTitleBarHeight, Child: props.Rail},
			{Child: props.TitleBar},
		}}
	} else {
		content := woxwidget.Container{Width: props.Width, Height: contentHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{props.Rail, page}}}
		bodyChild = woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{props.TitleBar, content}}
	}
	body := woxwidget.Container{Width: props.Width, Height: props.Height, Color: props.Theme.Background, Radius: props.Radius, Child: bodyChild}
	layers := []woxwidget.StackChild{{Child: body}}
	if props.Overlay != nil {
		layers = append(layers, woxwidget.StackChild{Left: props.OverlayLeft, Top: props.OverlayTop, Child: props.Overlay})
	}
	// Keep the root shape stable while transient overlays appear so retained hover identities stay mounted.
	window := woxwidget.Container{Width: props.Width, Height: props.Height, Radius: props.Radius, Child: woxwidget.Stack{Width: props.Width, Height: props.Height, Children: layers}}
	return woxwidget.Semantics{Key: "settings-window-key", AutomationID: "settings.window", Role: woxui.AccessibilityRoleWindow, Label: "Wox Settings", Child: window}
}

// SettingsTitleBarProps contains the title and native window actions.
type SettingsTitleBarProps struct {
	Width float32
	// RailWidth reserves the macOS settings rail; zero makes the title bar span the full window.
	RailWidth float32
	// CloseOnly hides platform minimize and zoom controls for preview title bars.
	CloseOnly  bool
	Title      string
	TitleWidth float32
	Platform   string
	AppIcon    *woxui.Image
	Theme      woxcomponent.Theme
	OnDrag     func()
	OnMinimize func()
	OnClose    func()
	// Active is true while this window is the key window. macOS traffic lights
	// stay gray until it is, matching AppKit.
	Active bool
}

// SettingsTitleBar builds the draggable settings title bar.
func SettingsTitleBar(props SettingsTitleBarProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: "settings-title-bar", Type: (*settingsTitleBarState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &settingsTitleBarState{} },
	}
}

type settingsTitleBarState struct {
	hovered string
	pressed string
}

// InitState starts the title bar without a hovered native control.
func (s *settingsTitleBarState) InitState(_ woxwidget.StateContext, _ any) {}

// DidUpdateWidget preserves hover while immutable title and theme props change.
func (s *settingsTitleBarState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

// Build keeps native-control hover painting inside the retained title bar.
func (s *settingsTitleBarState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(SettingsTitleBarProps)
	onHover := func(control string, inside bool) {
		context.SetState(func() {
			if inside {
				s.hovered = control
			} else if s.hovered == control {
				s.hovered = ""
			}
		})
	}
	onPress := func(control string, pressed bool) {
		context.SetState(func() {
			if pressed {
				s.pressed = control
			} else if s.pressed == control {
				s.pressed = ""
			}
		})
	}
	return buildSettingsTitleBar(props, s.hovered, s.pressed, onHover, onPress)
}

// Dispose releases no external title bar resources.
func (s *settingsTitleBarState) Dispose() {}

// buildSettingsTitleBar composes platform title controls from retained hover state.
func buildSettingsTitleBar(props SettingsTitleBarProps, hovered, pressed string, onHover, onPress func(string, bool)) woxwidget.Widget {
	height := SettingsTitleBarHeight
	titleStyle := woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}
	dragWidth := props.Width
	if props.Platform == "darwin" && props.RailWidth > 0 {
		dragWidth = props.RailWidth
	}
	dragArea := woxwidget.Gesture{ID: "settings-title-drag", OnDragStart: props.OnDrag, Child: woxwidget.Container{Width: dragWidth, Height: height}}
	children := make([]woxwidget.StackChild, 0, 7)
	if props.Platform == "darwin" && props.RailWidth > 0 {
		children = append(children, woxwidget.StackChild{Child: woxwidget.Container{Width: props.RailWidth, Height: height, Color: woxcomponent.TitleBarAlpha(props.Theme.ToolbarText, 9)}})
	}
	children = append(children, woxwidget.StackChild{Child: dragArea})
	switch props.Platform {
	case "darwin":
		macLight := func(id string, color woxui.Color, glyph string, glyphColor woxui.Color, hovered, pressed bool, onTap func()) woxwidget.Widget {
			return woxcomponent.MacTrafficLight(id, color, glyph, glyphColor, hovered, pressed, props.Active, props.Theme, onTap, onHover, onPress)
		}
		if props.CloseOnly {
			if props.RailWidth > 0 {
				children = append(children, woxwidget.StackChild{Left: props.RailWidth - 1, Child: woxwidget.Container{Width: 1, Height: height, Color: woxcomponent.TitleBarAlpha(props.Theme.ToolbarText, 26)}})
			}
			children = append(children, woxwidget.StackChild{Left: 13, Child: macLight("settings-window-close", woxui.Color{R: 255, G: 92, B: 95, A: 255}, "×", woxui.Color{R: 128, G: 47, B: 49, A: 255}, hovered == "mac-controls", pressed == "settings-window-close", props.OnClose)})
			break
		}
		children = append(children,
			woxwidget.StackChild{Left: max(float32(0), props.RailWidth-1), Child: woxwidget.Container{Width: 1, Height: height, Color: woxcomponent.TitleBarAlpha(props.Theme.ToolbarText, 26)}},
			woxwidget.StackChild{Left: 13, Child: macLight("settings-window-close", woxui.Color{R: 255, G: 92, B: 95, A: 255}, "×", woxui.Color{R: 128, G: 47, B: 49, A: 255}, hovered == "mac-controls", pressed == "settings-window-close", props.OnClose)},
			woxwidget.StackChild{Left: 36, Child: macLight("settings-window-minimize", woxui.Color{R: 250, G: 200, B: 0, A: 255}, "−", woxui.Color{R: 126, G: 100, B: 11, A: 255}, hovered == "mac-controls", pressed == "settings-window-minimize", props.OnMinimize)},
			woxwidget.StackChild{Left: 59, Child: macLight("settings-window-zoom", woxui.Color{R: 142, G: 142, B: 147, A: 255}, "", woxui.Color{}, false, false, nil)},
		)
	case "windows":
		if props.CloseOnly {
			if props.AppIcon != nil {
				children = append(children, woxwidget.StackChild{Left: 12, Top: 10, Child: woxwidget.Image{Source: props.AppIcon, Width: 20, Height: 20}})
			}
			children = append(children,
				woxwidget.StackChild{Left: 40, Top: 9, Right: 46, StretchWidth: true, Child: woxwidget.Container{Height: 24, Child: woxwidget.Text{Value: props.Title, Style: titleStyle, Color: props.Theme.ToolbarText}}},
				woxwidget.StackChild{AnchorBottom: true, StretchWidth: true, Child: woxwidget.Container{Height: 1, Color: woxcomponent.TitleBarAlpha(props.Theme.PreviewSplit, 76)}},
				woxwidget.StackChild{AnchorRight: true, Child: woxcomponent.WindowsTitleBarButton("settings-window-close", "×", true, hovered == "close", props.Theme, props.OnClose, onHover)},
			)
			break
		}
		if props.AppIcon != nil {
			children = append(children, woxwidget.StackChild{Left: 12, Top: 10, Child: woxwidget.Image{Source: props.AppIcon, Width: 20, Height: 20}})
		}
		children = append(children,
			woxwidget.StackChild{Left: 40, Top: 9, Right: 92, StretchWidth: true, Child: woxwidget.Container{Height: 24, Child: woxwidget.Text{Value: props.Title, Style: titleStyle, Color: props.Theme.ToolbarText}}},
			woxwidget.StackChild{AnchorBottom: true, StretchWidth: true, Child: woxwidget.Container{Height: 1, Color: woxcomponent.TitleBarAlpha(props.Theme.PreviewSplit, 76)}},
			woxwidget.StackChild{Right: 46, AnchorRight: true, Child: woxcomponent.WindowsTitleBarButton("settings-window-minimize", "−", false, hovered == "minimize", props.Theme, props.OnMinimize, onHover)},
			woxwidget.StackChild{AnchorRight: true, Child: woxcomponent.WindowsTitleBarButton("settings-window-close", "×", true, hovered == "close", props.Theme, props.OnClose, onHover)},
		)
	default:
		closeButton := woxcomponent.WindowsTitleBarButton("settings-window-close", "×", true, hovered == "close", props.Theme, props.OnClose, onHover)
		if props.Platform == "linux" {
			closeButton = woxcomponent.LinuxTitleBarCloseButton("settings-window-close", hovered == "close", props.Theme, props.OnClose, onHover)
		}
		children = append(children,
			woxwidget.StackChild{Left: max(float32(0), (props.Width-props.TitleWidth)/2), Top: 9, Child: woxwidget.Container{Width: props.TitleWidth, Height: 24, Child: woxwidget.Text{Value: props.Title, Style: titleStyle, Color: props.Theme.ToolbarText}}},
			woxwidget.StackChild{AnchorRight: true, Child: closeButton},
		)
	}
	return woxwidget.Stack{Width: props.Width, Height: height, Children: children}
}

// woxcomponent.LinuxTitleBarCloseButton uses a compact circular hover highlight to match common Linux chrome conventions.
// SettingsThemePageProps contains the active theme route's prepared body.
type SettingsThemePageProps struct {
	Width  float32
	Height float32
	Body   woxwidget.Widget
}

// SettingsThemePage lets the navigation rail own the route and matches Flutter's twenty-pixel page inset.
func SettingsThemePage(props SettingsThemePageProps) woxwidget.Widget {
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.UniformInsets(20), Child: props.Body}
}
