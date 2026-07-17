package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// HotkeySettingsProps contains prepared form rows for the hotkey settings page.
type HotkeySettingsProps struct {
	Width         float32
	Height        float32
	Theme         woxcomponent.Theme
	Available     bool
	Rows          []woxwidget.Widget
	RowsHeight    float32
	Scroll        float32
	Note          string
	OnScroll      func(float32)
	OnSetViewport func(float32)
}

// HotkeySettingsView builds the hotkey settings page.
func HotkeySettingsView(props HotkeySettingsProps) woxwidget.Widget {
	if !props.Available {
		return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.UniformInsets(36), Child: woxwidget.Text{
			Value: "Hotkey settings are unavailable.", Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle,
		}}
	}
	innerWidth := max(float32(0), props.Width-72)
	headerHeight := float32(74)
	noteHeight := float32(34)
	bodyHeight := max(float32(80), props.Height-60-headerHeight-noteHeight)
	if props.OnSetViewport != nil {
		props.OnSetViewport(bodyHeight)
	}
	body := woxwidget.Gesture{ID: "hotkey-settings-scroll", OnScroll: func(delta woxui.Point) {
		if props.OnScroll != nil {
			props.OnScroll(-delta.Y)
		}
	}, Child: woxwidget.ScrollView{
		Width: innerWidth, Height: bodyHeight, ContentHeight: max(bodyHeight, props.RowsHeight), Offset: props.Scroll,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: props.Rows},
	}}
	note := props.Note
	if note == "" {
		note = "Core records raw hotkeys; this page only owns focus and persisted values."
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 36, Top: 30, Right: 36, Bottom: 30}, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			woxcomponent.WoxPageHeader(woxcomponent.PageHeaderProps{Title: "Hotkeys", Description: "Global activation and reusable query launchers", Width: innerWidth, Height: headerHeight, TitleSize: 24, Gap: 7, Theme: props.Theme}),
			body,
			woxwidget.Container{Width: innerWidth, Height: noteHeight, Padding: woxwidget.Insets{Top: 10}, Child: woxwidget.Text{Value: note, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle}},
		},
	}}
}
