package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// SettingsWindowProps contains the prepared rail, page, and optional modal overlay.
type SettingsWindowProps struct {
	Width    float32
	Height   float32
	Radius   float32
	TitleBar woxwidget.Widget
	Rail     woxwidget.Widget
	Page     woxwidget.Widget
	Overlay  woxwidget.Widget
	Theme    woxcomponent.Theme
}

const SettingsTitleBarHeight = float32(40)

// SettingsWindow builds the shared settings window frame.
func SettingsWindow(props SettingsWindowProps) woxwidget.Widget {
	contentHeight := max(float32(0), props.Height-SettingsTitleBarHeight)
	content := woxwidget.Container{Width: props.Width, Height: contentHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{props.Rail, props.Page}}}
	body := woxwidget.Container{Width: props.Width, Height: props.Height, Color: props.Theme.Background, Radius: props.Radius, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{props.TitleBar, content}}}
	if props.Overlay == nil {
		return body
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: props.Radius, Child: woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{{Child: body}, {Child: props.Overlay}}}}
}

// SettingsTitleBarProps contains the title and native window actions.
type SettingsTitleBarProps struct {
	Width      float32
	RailWidth  float32
	Title      string
	TitleWidth float32
	Platform   string
	AppIcon    *woxui.Image
	Hovered    string
	Theme      woxcomponent.Theme
	OnDrag     func()
	OnMinimize func()
	OnClose    func()
	OnHover    func(string, bool)
}

// SettingsTitleBar builds the draggable settings title bar.
func SettingsTitleBar(props SettingsTitleBarProps) woxwidget.Widget {
	height := SettingsTitleBarHeight
	titleStyle := woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}
	dragArea := woxwidget.Gesture{ID: "settings-title-drag", OnDragStart: props.OnDrag, Child: woxwidget.Container{Width: props.Width, Height: height}}
	children := []woxwidget.StackChild{{Child: dragArea}}
	switch props.Platform {
	case "darwin":
		contentWidth := max(float32(0), props.Width-props.RailWidth)
		children = append(children,
			woxwidget.StackChild{Left: props.RailWidth + max(float32(0), (contentWidth-props.TitleWidth)/2), Top: 9, Child: woxwidget.Container{Width: props.TitleWidth, Height: 24, Child: woxwidget.Text{Value: props.Title, Style: titleStyle, Color: props.Theme.ToolbarText}}},
			woxwidget.StackChild{Left: max(float32(0), props.RailWidth-1), Child: woxwidget.Container{Width: 1, Height: height, Color: settingsTitleBarAlpha(props.Theme.PreviewSplit, 128)}},
			woxwidget.StackChild{Left: 10, Child: settingsMacTrafficLight("settings-window-close", woxui.Color{R: 255, G: 95, B: 87, A: 255}, woxui.Color{R: 224, G: 68, B: 62, A: 255}, "×", woxui.Color{R: 126, G: 29, B: 24, A: 255}, props.Hovered == "mac-controls", props.OnClose, props.OnHover)},
			woxwidget.StackChild{Left: 33, Child: settingsMacTrafficLight("settings-window-minimize", woxui.Color{R: 255, G: 189, B: 46, A: 255}, woxui.Color{R: 223, G: 160, B: 35, A: 255}, "−", woxui.Color{R: 138, G: 90, B: 0, A: 255}, props.Hovered == "mac-controls", props.OnMinimize, props.OnHover)},
			woxwidget.StackChild{Left: 56, Child: settingsMacTrafficLight("settings-window-zoom", woxui.Color{R: 142, G: 142, B: 147, A: 255}, woxui.Color{R: 119, G: 119, B: 124, A: 255}, "", woxui.Color{}, props.Hovered == "mac-controls", nil, props.OnHover)},
		)
	case "windows":
		if props.AppIcon != nil {
			children = append(children, woxwidget.StackChild{Left: 12, Top: 10, Child: woxwidget.Image{Source: props.AppIcon, Width: 20, Height: 20}})
		}
		children = append(children,
			woxwidget.StackChild{Left: 40, Top: 9, Child: woxwidget.Container{Width: max(float32(0), props.Width-132), Height: 24, Child: woxwidget.Text{Value: props.Title, Style: titleStyle, Color: props.Theme.ToolbarText}}},
			woxwidget.StackChild{Top: height - 1, Child: woxwidget.Container{Width: props.Width, Height: 1, Color: settingsTitleBarAlpha(props.Theme.PreviewSplit, 76)}},
			woxwidget.StackChild{Left: max(float32(0), props.Width-92), Child: settingsWindowsTitleBarButton("settings-window-minimize", "−", false, props.Hovered == "minimize", props.Theme, props.OnMinimize, props.OnHover)},
			woxwidget.StackChild{Left: max(float32(0), props.Width-46), Child: settingsWindowsTitleBarButton("settings-window-close", "×", true, props.Hovered == "close", props.Theme, props.OnClose, props.OnHover)},
		)
	default:
		children = append(children,
			woxwidget.StackChild{Left: max(float32(0), (props.Width-props.TitleWidth)/2), Top: 9, Child: woxwidget.Container{Width: props.TitleWidth, Height: 24, Child: woxwidget.Text{Value: props.Title, Style: titleStyle, Color: props.Theme.ToolbarText}}},
			woxwidget.StackChild{Left: max(float32(0), props.Width-46), Child: settingsWindowsTitleBarButton("settings-window-close", "×", false, props.Hovered == "close", props.Theme, props.OnClose, props.OnHover)},
		)
	}
	return woxwidget.Stack{Width: props.Width, Height: height, Children: children}
}

// settingsWindowsTitleBarButton matches the compact native hover treatment while keeping the frameless window fully custom-drawn.
func settingsWindowsTitleBarButton(id, glyph string, closeButton, hovered bool, theme woxcomponent.Theme, onTap func(), onHover func(string, bool)) woxwidget.Widget {
	background := woxui.Color{}
	foreground := settingsTitleBarAlpha(theme.ToolbarText, 230)
	if hovered {
		background = settingsTitleBarAlpha(theme.ToolbarText, 26)
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
	}, Child: woxwidget.Container{Width: 46, Height: SettingsTitleBarHeight, Color: background, Child: woxwidget.Align{Width: 46, Height: SettingsTitleBarHeight, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: glyph, Style: woxui.TextStyle{Size: 18}, Color: foreground}}}}
}

// settingsMacTrafficLight keeps disabled traffic lights visible and only reveals control glyphs while the group is hovered.
func settingsMacTrafficLight(id string, color, border woxui.Color, glyph string, glyphColor woxui.Color, hovered bool, onTap func(), onHover func(string, bool)) woxwidget.Widget {
	label := ""
	if hovered && onTap != nil {
		label = glyph
	}
	return woxwidget.Gesture{ID: id, OnTap: onTap, OnHover: func(inside bool) {
		if onHover != nil {
			onHover("mac-controls", inside)
		}
	}, Child: woxwidget.Align{Width: 20, Height: SettingsTitleBarHeight, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Container{Width: 14, Height: 14, Radius: 7, Color: color, BorderColor: border, BorderWidth: 1, Child: woxwidget.Align{Width: 14, Height: 14, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 10, Weight: woxui.FontWeightSemibold}, Color: glyphColor}}}}}
}

func settingsTitleBarAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}

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
