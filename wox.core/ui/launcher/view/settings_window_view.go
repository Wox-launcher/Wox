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
	Theme       woxcomponent.ControlTheme
}

const SettingsTitleBarHeight = woxcomponent.TitleBarHeight

// SettingsPageTop keeps page placement and its available height in sync with platform chrome.
func SettingsPageTop(platform string) float32 {
	switch platform {
	case "darwin":
		return 0
	case "windows":
		// Catalogs inset controls by 20 units, so only their empty gutter overlaps the caption row.
		return SettingsTitleBarHeight - 20
	default:
		return SettingsTitleBarHeight
	}
}

// SettingsWindow builds the shared settings window frame.
func SettingsWindow(props SettingsWindowProps) woxwidget.Widget {
	contentHeight := max(float32(0), props.Height-SettingsTitleBarHeight)
	page := woxwidget.Semantics{
		Key: "settings-page-key", AutomationID: "settings.page." + props.PageID, Role: woxui.AccessibilityRoleGroup, Label: props.PageID + " settings",
		Child: props.Page,
	}
	var bodyChild woxwidget.Widget
	if props.Platform == "darwin" || props.Platform == "windows" {
		// Let the page's top gutter share the chrome area instead of stacking both blank spaces.
		bodyChild = woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
			{Left: props.RailWidth, Top: SettingsPageTop(props.Platform), Child: page},
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
	// RailWidth continues the settings rail's surface into the title bar; macOS also limits dragging to the rail.
	RailWidth float32
	// CloseOnly hides platform minimize and zoom controls for preview title bars.
	CloseOnly  bool
	Title      string
	TitleWidth float32
	// Content replaces the ordinary title while preserving platform window controls.
	Content    woxwidget.Widget
	Platform   string
	AppIcon    *woxui.Image
	Theme      woxcomponent.ControlTheme
	OnDrag     func()
	OnMinimize func()
	OnClose    func()
	// Active is true while this window is the key window. macOS traffic lights
	// stay gray until it is, matching AppKit.
	Active bool
}

// TitleBarContentFrame returns the area available beside platform window controls.
func TitleBarContentFrame(platform string, closeOnly bool, width float32) (left, contentWidth float32) {
	if !closeOnly {
		return 0, width
	}
	left = 0
	right := float32(46)
	if platform == "darwin" {
		left = 44
		right = 0
	}
	return left, max(float32(0), width-left-right)
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
	return buildSettingsTitleBar(props, s.hovered, onHover)
}

// Dispose releases no external title bar resources.
func (s *settingsTitleBarState) Dispose() {}

// buildSettingsTitleBar composes platform title controls from retained hover state.
func buildSettingsTitleBar(props SettingsTitleBarProps, hovered string, onHover func(string, bool)) woxwidget.Widget {
	height := SettingsTitleBarHeight
	titleStyle := woxui.TextStyle{Size: props.Theme.Scaled(13), Weight: woxui.FontWeightSemibold}
	dragWidth := props.Width
	if props.Platform == "darwin" && props.RailWidth > 0 {
		dragWidth = props.RailWidth
	}
	dragArea := woxwidget.Gesture{ID: "settings-title-drag", OnDragStart: props.OnDrag, Child: woxwidget.Container{Width: dragWidth, Height: height}}
	children := make([]woxwidget.StackChild, 0, 7)
	if (props.Platform == "darwin" || props.Platform == "windows") && props.RailWidth > 0 {
		// Continue the rail tint above its search field so the title bar has no horizontal color seam.
		children = append(children, woxwidget.StackChild{Child: woxwidget.Container{Width: props.RailWidth, Height: height, Color: woxcomponent.TitleBarAlpha(props.Theme.TextSecondary, 9)}})
	}
	children = append(children, woxwidget.StackChild{Child: dragArea})
	if props.Content != nil {
		left, contentWidth := TitleBarContentFrame(props.Platform, props.CloseOnly, props.Width)
		children = append(children, woxwidget.StackChild{Left: left, Child: woxwidget.Clip{Width: contentWidth, Height: height, Child: props.Content}})
	}
	switch props.Platform {
	case "darwin":
		// Native captions stay above the Go surface. Keep only the rail separator here.
		if !props.CloseOnly || props.RailWidth > 0 {
			children = append(children, woxwidget.StackChild{Left: max(float32(0), props.RailWidth-1), Child: woxwidget.Container{Width: 1, Height: height, Color: woxcomponent.TitleBarAlpha(props.Theme.TextSecondary, 26)}})
		}
	case "windows":
		if props.RailWidth > 0 {
			children = append(children, woxwidget.StackChild{Left: max(float32(0), props.RailWidth-1), Child: woxwidget.Container{Width: 1, Height: height, Color: woxcomponent.TitleBarAlpha(props.Theme.TextSecondary, 26)}})
		}
		if props.CloseOnly {
			if props.Content == nil && props.AppIcon != nil {
				children = append(children, woxwidget.StackChild{Left: 12, Child: woxwidget.Align{Width: 20, Height: height, Vertical: 0.5, Child: woxwidget.Image{Source: props.AppIcon, Width: 20, Height: 20}}})
			}
			if props.Content == nil {
				children = append(children, woxwidget.StackChild{Left: 40, Right: 46, StretchWidth: true, Child: woxwidget.Align{Height: height, Vertical: 0.5, Child: woxwidget.Text{Value: props.Title, Style: titleStyle, Color: props.Theme.TextSecondary}}})
			}
			children = append(children,
				woxwidget.StackChild{AnchorRight: true, Child: woxcomponent.WindowsTitleBarButton("settings-window-close", "close", hovered == "close", props.Theme, props.OnClose, onHover)},
			)
			break
		}
		if props.AppIcon != nil {
			children = append(children, woxwidget.StackChild{Left: 12, Child: woxwidget.Align{Width: 20, Height: height, Vertical: 0.5, Child: woxwidget.Image{Source: props.AppIcon, Width: 20, Height: 20}}})
		}
		children = append(children,
			woxwidget.StackChild{Left: 40, Right: 92, StretchWidth: true, Child: woxwidget.Align{Height: height, Vertical: 0.5, Child: woxwidget.Text{Value: props.Title, Style: titleStyle, Color: props.Theme.TextSecondary}}},
			woxwidget.StackChild{Right: 46, AnchorRight: true, Child: woxcomponent.WindowsTitleBarButton("settings-window-minimize", "minimize", hovered == "minimize", props.Theme, props.OnMinimize, onHover)},
			woxwidget.StackChild{AnchorRight: true, Child: woxcomponent.WindowsTitleBarButton("settings-window-close", "close", hovered == "close", props.Theme, props.OnClose, onHover)},
		)
	default:
		closeButton := woxcomponent.WindowsTitleBarButton("settings-window-close", "close", hovered == "close", props.Theme, props.OnClose, onHover)
		if props.Platform == "linux" {
			closeButton = woxcomponent.LinuxTitleBarCloseButton("settings-window-close", hovered == "close", props.Theme, props.OnClose, onHover)
		}
		if props.Content == nil {
			children = append(children, woxwidget.StackChild{Left: max(float32(0), (props.Width-props.TitleWidth)/2), Child: woxwidget.Align{Width: props.TitleWidth, Height: height, Vertical: 0.5, Child: woxwidget.Text{Value: props.Title, Style: titleStyle, Color: props.Theme.TextSecondary}}})
		}
		children = append(children, woxwidget.StackChild{AnchorRight: true, Child: closeButton})
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
