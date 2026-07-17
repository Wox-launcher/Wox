package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// AboutLink contains one external About destination.
type AboutLink struct {
	ID    string
	Label string
	OnTap func()
}

// AboutSettingsProps contains the running version and external destinations.
type AboutSettingsProps struct {
	Width   float32
	Height  float32
	Version string
	Status  string
	Links   []AboutLink
	Theme   woxcomponent.Theme
}

// AboutSettingsView builds the About settings route.
func AboutSettingsView(props AboutSettingsProps) woxwidget.Widget {
	contentWidth := min(float32(640), max(float32(0), props.Width-96))
	left := max(float32(48), (props.Width-contentWidth)*0.5)
	links := make([]woxwidget.Widget, 0, len(props.Links))
	for _, link := range props.Links {
		links = append(links, woxwidget.Gesture{ID: "about-link-" + link.ID, OnTap: link.OnTap, Child: woxwidget.Container{
			Width: 150, Height: 42, Radius: 9, Color: props.Theme.QueryBackground, Padding: woxwidget.Insets{Left: 18, Top: 12},
			Child: woxwidget.Text{Value: link.Label + "  ↗", Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.Cursor},
		}})
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: left, Top: 92, Right: left, Bottom: 40}, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 22, Children: []woxwidget.Widget{
			woxwidget.Container{Width: contentWidth, Height: 86, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{
				woxwidget.Text{Value: "WOX", Style: woxui.TextStyle{Size: 42, Weight: woxui.FontWeightSemibold}, Color: props.Theme.QueryText},
				woxwidget.Text{Value: props.Version, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.Cursor},
			}}},
			woxwidget.TextBlock{Value: "A cross-platform launcher that keeps plugins, search, automation, and AI workflows one keystroke away.", Width: contentWidth, Height: 64, Style: woxui.TextStyle{Size: 16}, LineHeight: 24, Color: props.Theme.ResultTitle},
			woxwidget.Container{Width: contentWidth, Height: 44, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 14, Children: links}},
			woxwidget.Text{Value: props.Status, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ErrorText},
		},
	}}
}
