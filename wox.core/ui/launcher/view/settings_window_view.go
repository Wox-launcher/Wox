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

// SettingsWindow builds the shared settings window frame.
func SettingsWindow(props SettingsWindowProps) woxwidget.Widget {
	contentHeight := max(float32(0), props.Height-42)
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
	Title      string
	TitleWidth float32
	ShowClose  bool
	Theme      woxcomponent.Theme
	OnDrag     func()
	OnClose    func()
}

// SettingsTitleBar builds the draggable settings title bar.
func SettingsTitleBar(props SettingsTitleBarProps) woxwidget.Widget {
	const height = float32(42)
	titleStyle := woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}
	dragArea := woxwidget.Gesture{ID: "settings-title-drag", OnDragStart: props.OnDrag, Child: woxwidget.Container{Width: props.Width, Height: height}}
	children := []woxwidget.StackChild{
		{Child: dragArea},
		{Left: max(float32(0), (props.Width-props.TitleWidth)/2), Top: 12, Child: woxwidget.Container{Width: props.TitleWidth, Height: 24, Child: woxwidget.Text{Value: props.Title, Style: titleStyle, Color: props.Theme.ToolbarText}}},
	}
	if props.ShowClose {
		const closeWidth = float32(52)
		children = append(children, woxwidget.StackChild{Left: max(float32(0), props.Width-closeWidth), Child: woxwidget.Gesture{ID: "settings-window-close", OnTap: props.OnClose, Child: woxwidget.Container{
			Width: closeWidth, Height: height, Padding: woxwidget.Insets{Left: 20, Top: 10}, Child: woxwidget.Text{Value: "×", Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ToolbarText},
		}}})
	}
	return woxwidget.Stack{Width: props.Width, Height: height, Children: children}
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
