package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// WebViewTitleBarProps contains browser navigation and native window actions.
type WebViewTitleBarProps struct {
	Width              float32
	Platform           string
	AppIcon            *woxui.Image
	Theme              woxcomponent.Theme
	URL                string
	CanGoBack          bool
	CanGoForward       bool
	GoBackLabel        string
	RefreshLabel       string
	GoForwardLabel     string
	OpenInBrowserLabel string
	OnDrag             func()
	OnClose            func()
	OnGoBack           func()
	OnGoForward        func()
	OnRefresh          func()
	OnOpenInBrowser    func()
}

type webViewTitleBarState struct {
	closeHovered bool
	closePressed bool
}

// WebViewTitleBar builds browser-like chrome for a native WebView preview.
func WebViewTitleBar(props WebViewTitleBarProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: "webview-title-bar", Type: (*webViewTitleBarState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &webViewTitleBarState{} },
	}
}

func (s *webViewTitleBarState) InitState(_ woxwidget.StateContext, _ any) {}

func (s *webViewTitleBarState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

// Build keeps native close-button hover painting in retained state.
func (s *webViewTitleBarState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(WebViewTitleBarProps)
	onHover := func(control string, inside bool) {
		if control != "close" && control != "mac-controls" {
			return
		}
		context.SetState(func() { s.closeHovered = inside })
	}
	onPress := func(control string, pressed bool) {
		if control != "webview-window-close" {
			return
		}
		context.SetState(func() { s.closePressed = pressed })
	}
	return buildWebViewTitleBar(props, s.closeHovered, s.closePressed, onHover, onPress)
}

func (s *webViewTitleBarState) Dispose() {}

// buildWebViewTitleBar composes navigation, location, and platform close controls.
func buildWebViewTitleBar(props WebViewTitleBarProps, closeHovered, closePressed bool, onHover, onPress func(string, bool)) woxwidget.Widget {
	const (
		buttonSize = float32(32)
		buttonGap  = float32(2)
		sideGap    = float32(8)
	)
	height := SettingsTitleBarHeight
	foreground := settingsTitleBarAlpha(props.Theme.ToolbarText, 230)
	disabledForeground := settingsTitleBarAlpha(props.Theme.ToolbarText, 90)
	hoverBackground := settingsTitleBarAlpha(props.Theme.ToolbarText, 20)
	omniboxBackground := settingsTitleBarAlpha(props.Theme.ToolbarText, 14)
	borderColor := settingsTitleBarAlpha(props.Theme.PreviewSplit, 76)

	navLeft := float32(6)
	if props.Platform == "darwin" {
		navLeft = 42
	} else if props.AppIcon != nil {
		navLeft = 40
	}
	navWidth := buttonSize*3 + buttonGap*2
	closeLeft := max(float32(0), props.Width-46)
	openLeft := max(navLeft+navWidth+sideGap, closeLeft-sideGap-buttonSize)
	omniboxLeft := navLeft + navWidth + sideGap
	omniboxWidth := max(float32(0), openLeft-sideGap-omniboxLeft)

	iconButton := func(id, label string, icon woxwidget.Widget, left float32, disabled bool, onTap func()) woxwidget.StackChild {
		return woxwidget.StackChild{Left: left, Top: 4, Child: woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
			ID: id, Label: label, Icon: icon, Width: buttonSize, Height: buttonSize, Radius: 5,
			HoverBackground: hoverBackground, FocusRingColor: props.Theme.Cursor, Disabled: disabled, OnTap: onTap,
		})}
	}
	backColor := foreground
	if !props.CanGoBack {
		backColor = disabledForeground
	}
	forwardColor := foreground
	if !props.CanGoForward {
		forwardColor = disabledForeground
	}

	children := []woxwidget.StackChild{
		{Child: woxwidget.Gesture{ID: "webview-title-drag", OnDragStart: props.OnDrag, Child: woxwidget.Container{Width: props.Width, Height: height}}},
		{Top: height - 1, Child: woxwidget.Container{Width: props.Width, Height: 1, Color: borderColor}},
	}
	if props.Platform != "darwin" && props.AppIcon != nil {
		children = append(children, woxwidget.StackChild{Left: 12, Top: 10, Child: woxwidget.Image{Source: props.AppIcon, Width: 20, Height: 20}})
	}
	children = append(children,
		iconButton("webview-go-back", props.GoBackLabel, woxcomponent.ArrowLeftGlyph(16, backColor), navLeft, !props.CanGoBack, props.OnGoBack),
		iconButton("webview-go-forward", props.GoForwardLabel, woxcomponent.ArrowRightGlyph(16, forwardColor), navLeft+buttonSize+buttonGap, !props.CanGoForward, props.OnGoForward),
		iconButton("webview-refresh", props.RefreshLabel, woxcomponent.RefreshGlyph(15, foreground), navLeft+(buttonSize+buttonGap)*2, false, props.OnRefresh),
		woxwidget.StackChild{Left: omniboxLeft, Top: 6, Child: woxwidget.Semantics{
			AutomationID: "webview-location", Role: woxui.AccessibilityRoleText, Label: props.URL,
			Child: woxwidget.Container{
				Width: omniboxWidth, Height: 28, Radius: 7, Color: omniboxBackground, Padding: woxwidget.Insets{Left: 10, Right: 10, Top: 5},
				Child: woxwidget.Clip{Width: max(float32(0), omniboxWidth-20), Height: 18, Child: woxwidget.Text{Value: props.URL, Style: woxui.TextStyle{Size: 12}, Color: settingsTitleBarAlpha(props.Theme.ToolbarText, 205)}},
			},
		}},
		iconButton("webview-open-in-browser", props.OpenInBrowserLabel, woxcomponent.ExternalGlyph(15, foreground), openLeft, false, props.OnOpenInBrowser),
	)

	switch props.Platform {
	case "darwin":
		children = append(children, woxwidget.StackChild{Left: 13, Child: settingsMacTrafficLight(
			"webview-window-close", woxui.Color{R: 255, G: 92, B: 95, A: 255}, "×", woxui.Color{R: 128, G: 47, B: 49, A: 255},
			closeHovered, closePressed, props.OnClose, onHover, onPress,
		)})
	case "linux":
		children = append(children, woxwidget.StackChild{Left: closeLeft, Child: settingsLinuxTitleBarCloseButton("webview-window-close", closeHovered, props.Theme, props.OnClose, onHover)})
	default:
		children = append(children, woxwidget.StackChild{Left: closeLeft, Child: settingsWindowsTitleBarButton("webview-window-close", "×", true, closeHovered, props.Theme, props.OnClose, onHover)})
	}
	return woxwidget.Stack{Width: props.Width, Height: height, Children: children}
}
