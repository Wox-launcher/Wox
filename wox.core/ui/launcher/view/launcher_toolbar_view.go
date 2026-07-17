package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// LauncherToolbarAction contains one translated toolbar action.
type LauncherToolbarAction struct {
	ID           string
	Label        string
	HotkeyLabels []string
	OnTap        func()
}

// LauncherToolbarProps contains the launcher status and available result actions.
type LauncherToolbarProps struct {
	Width         float32
	Height        float32
	Padding       woxwidget.Insets
	Theme         woxcomponent.Theme
	Window        *woxui.Window
	Label         string
	Icon          *woxui.Image
	ProgressLabel string
	Actions       []LauncherToolbarAction
}

type measuredLauncherToolbarAction struct {
	widget woxwidget.Widget
	width  float32
}

// LauncherToolbarView builds the status footer and the actions that fit its current width.
func LauncherToolbarView(props LauncherToolbarProps) woxwidget.Widget {
	contentWidth := max(float32(0), props.Width-props.Padding.Left-props.Padding.Right)
	leftWidth := float32(0)
	if props.Label != "" || props.Icon != nil || props.ProgressLabel != "" {
		leftWidth = min(contentWidth*0.42, float32(320))
	}
	rightAvailable := max(float32(0), contentWidth-leftWidth)
	if leftWidth > 0 && len(props.Actions) > 0 {
		rightAvailable -= 16
	}
	measured := make([]measuredLauncherToolbarAction, 0, len(props.Actions))
	for _, action := range props.Actions {
		widget, width := launcherToolbarActionView(action, props.Theme, props.Window)
		measured = append(measured, measuredLauncherToolbarAction{widget: widget, width: width})
	}
	shown := make([]measuredLauncherToolbarAction, 0, len(measured))
	rightWidth := float32(0)
	for index := len(measured) - 1; index >= 0; index-- {
		nextWidth := measured[index].width
		if len(shown) > 0 {
			nextWidth += 16
		}
		if rightWidth+nextWidth > rightAvailable {
			break
		}
		rightWidth += nextWidth
		shown = append([]measuredLauncherToolbarAction{measured[index]}, shown...)
	}
	rightChildren := make([]woxwidget.Widget, 0, len(shown))
	for _, action := range shown {
		rightChildren = append(rightChildren, action.widget)
	}
	extraWidth := float32(0)
	if props.Icon != nil {
		extraWidth += 26
	}
	progressWidth := float32(0)
	if props.ProgressLabel != "" {
		metrics, _ := props.Window.MeasureText(props.ProgressLabel, woxui.TextStyle{Size: 12})
		progressWidth = min(float32(90), metrics.Size.Width+4)
		extraWidth += progressWidth + 8
	}
	labelWidth := max(float32(0), leftWidth-extraWidth)
	leftWidgets := make([]woxwidget.Widget, 0, 3)
	if props.Icon != nil {
		leftWidgets = append(leftWidgets, woxwidget.Container{
			Width: 18, Height: 28, Padding: woxwidget.Insets{Top: 5}, Child: woxwidget.Image{Source: props.Icon, Width: 18, Height: 18},
		})
	}
	leftWidgets = append(leftWidgets, woxwidget.Container{
		Width: labelWidth, Height: 28, Padding: woxwidget.Insets{Top: 7},
		Child: woxwidget.Text{Value: props.Label, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ToolbarText},
	})
	if props.ProgressLabel != "" {
		leftWidgets = append(leftWidgets, woxwidget.Container{
			Width: progressWidth, Height: 28, Padding: woxwidget.Insets{Top: 7},
			Child: woxwidget.Text{Value: props.ProgressLabel, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: props.Theme.Cursor},
		})
	}
	body := woxwidget.Container{
		Width: props.Width, Height: props.Height, Color: props.Theme.ToolbarBackground,
		Padding: woxwidget.Insets{Left: props.Padding.Left, Top: 6, Right: props.Padding.Right, Bottom: 6},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: leftWidth, Height: 28, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: leftWidgets}},
			woxwidget.Painter{Width: max(float32(0), contentWidth-leftWidth-rightWidth), Height: 1},
			woxwidget.Container{Width: rightWidth, Height: 28, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, Children: rightChildren}},
		}},
	}
	border := props.Theme.ToolbarText
	border.A = min(border.A, uint8(26))
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
		{Child: body},
		{Child: woxwidget.Painter{Width: props.Width, Height: 1, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			displayList.FillRect(bounds, border)
		}}},
	}}
}

// launcherToolbarActionView builds one label-and-keycap unit and reports its width.
func launcherToolbarActionView(action LauncherToolbarAction, theme woxcomponent.Theme, window *woxui.Window) (woxwidget.Widget, float32) {
	labelStyle := woxui.TextStyle{Size: 12}
	labelMetrics, _ := window.MeasureText(action.Label, labelStyle)
	chip, chipWidth := woxcomponent.WoxHotkey(woxcomponent.HotkeyProps{
		Labels: action.HotkeyLabels, Foreground: theme.ToolbarText, Background: theme.ToolbarBackground, Window: window,
	})
	width := labelMetrics.Size.Width + 8 + chipWidth
	content := woxwidget.Container{Width: width, Height: 28, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Container{Width: labelMetrics.Size.Width, Height: 28, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{Value: action.Label, Style: labelStyle, Color: theme.ToolbarText}},
		chip,
	}}}
	return woxwidget.Gesture{ID: action.ID, OnTap: action.OnTap, Child: content}, width
}
