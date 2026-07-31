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
	DensityScale  float32
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
	contentHeight := scaledLauncherSize(28, props.DensityScale)
	fontSize := scaledLauncherSize(12, props.DensityScale)
	actionGap := scaledLauncherSize(16, props.DensityScale)
	contentWidth := max(float32(0), props.Width-props.Padding.Left-props.Padding.Right)
	leftWidth := float32(0)
	if props.Label != "" || props.Icon != nil || props.ProgressLabel != "" {
		leftWidth = min(contentWidth*0.42, scaledLauncherSize(320, props.DensityScale))
	}
	rightAvailable := max(float32(0), contentWidth-leftWidth)
	if leftWidth > 0 && len(props.Actions) > 0 {
		rightAvailable -= actionGap
	}
	measured := make([]measuredLauncherToolbarAction, 0, len(props.Actions))
	for _, action := range props.Actions {
		widget, width := launcherToolbarActionView(action, props.Theme, props.Window, props.DensityScale)
		measured = append(measured, measuredLauncherToolbarAction{widget: widget, width: width})
	}
	shown := make([]measuredLauncherToolbarAction, 0, len(measured))
	rightWidth := float32(0)
	for index := len(measured) - 1; index >= 0; index-- {
		nextWidth := measured[index].width
		if len(shown) > 0 {
			nextWidth += actionGap
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
		extraWidth += scaledLauncherSize(26, props.DensityScale)
	}
	progressWidth := float32(0)
	if props.ProgressLabel != "" {
		metrics, _ := props.Window.MeasureText(props.ProgressLabel, woxui.TextStyle{Size: fontSize})
		progressWidth = min(scaledLauncherSize(90, props.DensityScale), metrics.Size.Width+scaledLauncherSize(4, props.DensityScale))
		extraWidth += progressWidth + scaledLauncherSize(8, props.DensityScale)
	}
	labelWidth := max(float32(0), leftWidth-extraWidth)
	leftWidgets := make([]woxwidget.Widget, 0, 3)
	if props.Icon != nil {
		iconSize := scaledLauncherSize(18, props.DensityScale)
		leftWidgets = append(leftWidgets, woxwidget.Container{
			Width: iconSize, Height: contentHeight, Padding: woxwidget.Insets{Top: max(float32(0), (contentHeight-iconSize)/2)}, Child: woxwidget.Image{Source: props.Icon, Width: iconSize, Height: iconSize},
		})
	}
	leftWidgets = append(leftWidgets, woxwidget.Container{
		Width: labelWidth, Height: contentHeight, Padding: woxwidget.Insets{Top: scaledLauncherSize(7, props.DensityScale)},
		Child: woxwidget.Text{Value: props.Label, Style: woxui.TextStyle{Size: fontSize}, Color: props.Theme.ToolbarText},
	})
	if props.ProgressLabel != "" {
		leftWidgets = append(leftWidgets, woxwidget.Container{
			Width: progressWidth, Height: contentHeight, Padding: woxwidget.Insets{Top: scaledLauncherSize(7, props.DensityScale)},
			Child: woxwidget.Text{Value: props.ProgressLabel, Style: woxui.TextStyle{Size: fontSize, Weight: woxui.FontWeightSemibold}, Color: props.Theme.Cursor},
		})
	}
	verticalPadding := max(float32(0), (props.Height-contentHeight)/2)
	body := woxwidget.Container{
		Width: props.Width, Height: props.Height, Color: props.Theme.ToolbarBackground,
		Padding: woxwidget.Insets{Left: props.Padding.Left, Top: verticalPadding, Right: props.Padding.Right, Bottom: verticalPadding},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: leftWidth, Height: contentHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: scaledLauncherSize(8, props.DensityScale), Children: leftWidgets}},
			woxwidget.Painter{Width: max(float32(0), contentWidth-leftWidth-rightWidth), Height: 1},
			woxwidget.Container{Width: rightWidth, Height: contentHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: actionGap, Children: rightChildren}},
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
func launcherToolbarActionView(action LauncherToolbarAction, theme woxcomponent.Theme, window *woxui.Window, densityScale float32) (woxwidget.Widget, float32) {
	labelStyle := woxui.TextStyle{Size: scaledLauncherSize(12, densityScale)}
	labelMetrics, _ := window.MeasureText(action.Label, labelStyle)
	chip, chipWidth := woxcomponent.WoxHotkey(woxcomponent.HotkeyProps{
		Labels: action.HotkeyLabels, Foreground: theme.ToolbarText, Background: theme.ToolbarBackground, Compact: densityScale < 1, Window: window,
	})
	contentHeight := scaledLauncherSize(28, densityScale)
	gap := scaledLauncherSize(8, densityScale)
	width := labelMetrics.Size.Width + gap + chipWidth
	content := woxwidget.Container{Width: width, Height: contentHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, Children: []woxwidget.Widget{
		woxwidget.Container{Width: labelMetrics.Size.Width, Height: contentHeight, Padding: woxwidget.Insets{Top: scaledLauncherSize(7, densityScale)}, Child: woxwidget.Text{Value: action.Label, Style: labelStyle, Color: theme.ToolbarText}},
		chip,
	}}}
	return woxwidget.Gesture{ID: action.ID, OnTap: action.OnTap, Child: content}, width
}
