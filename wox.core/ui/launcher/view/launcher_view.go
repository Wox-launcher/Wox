package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// LauncherFloatingView contains one positioned launcher panel.
type LauncherFloatingView struct {
	Child woxwidget.Widget
	Left  float32
	Top   float32
}

// LauncherViewProps contains the prepared launcher sections and overlays.
type LauncherViewProps struct {
	Width         float32
	Height        float32
	Radius        float32
	Header        woxwidget.Widget
	Refinements   woxwidget.Widget
	Content       woxwidget.Widget
	Footer        woxwidget.Widget
	QueryAtBottom bool
	Floating      *LauncherFloatingView
	Overlay       woxwidget.Widget
	Theme         woxcomponent.Theme
}

// LauncherView builds the accessible launcher window and its overlay layers.
func LauncherView(props LauncherViewProps) woxwidget.Widget {
	sections := make([]woxwidget.Widget, 0, 4)
	if !props.QueryAtBottom {
		if props.Header != nil {
			sections = append(sections, props.Header)
		}
		if props.Refinements != nil {
			sections = append(sections, props.Refinements)
		}
	}
	if props.Content != nil {
		sections = append(sections, props.Content)
	}
	if props.QueryAtBottom {
		if props.Refinements != nil {
			sections = append(sections, props.Refinements)
		}
		if props.Header != nil {
			sections = append(sections, props.Header)
		}
	}
	if props.Footer != nil {
		sections = append(sections, props.Footer)
	}
	body := woxwidget.Widget(woxwidget.Flex{Axis: woxwidget.Vertical, Children: sections})
	if props.Floating != nil && props.Floating.Child != nil {
		body = woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
			{Child: body},
			{Left: props.Floating.Left, Top: props.Floating.Top, Child: props.Floating.Child},
		}}
	}
	if props.Overlay != nil {
		body = woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{{Child: body}, {Child: props.Overlay}}}
	}
	return woxwidget.Semantics{
		Key: "launcher-window-key", AutomationID: "launcher.window", Role: woxui.AccessibilityRoleWindow, Label: "Wox",
		Child: woxwidget.Container{Width: props.Width, Height: props.Height, Color: props.Theme.Background, Radius: props.Radius, Child: body},
	}
}
