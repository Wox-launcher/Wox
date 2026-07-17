package view

import (
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// PluginDetailPreviewProps contains resolved resources for a plugin store preview.
type PluginDetailPreviewProps struct {
	Width       float32
	Height      float32
	Theme       woxcomponent.Theme
	Name        string
	Description string
	Author      string
	Version     string
	Runtime     string
	Website     string
	Icon        *woxui.Image
	HasIcon     bool
	Screenshot  woxwidget.Widget
}

// PluginDetailPreviewView builds plugin metadata and its optional first screenshot.
func PluginDetailPreviewView(props PluginDetailPreviewProps) woxwidget.Widget {
	headerHeight := min(float32(108), props.Height)
	iconWidth := float32(0)
	var icon woxwidget.Widget
	if props.HasIcon {
		iconWidth = 62
		if props.Icon != nil {
			icon = woxwidget.Container{Width: 50, Height: 50, Radius: 9, Color: props.Theme.Background, Padding: woxwidget.UniformInsets(7), Child: woxwidget.Image{Source: props.Icon, Width: 36, Height: 36}}
		} else {
			icon = woxwidget.Container{Width: 50, Height: 50, Radius: 9, Color: props.Theme.Background}
		}
	}
	metadata := make([]string, 0, 4)
	if props.Author != "" {
		metadata = append(metadata, props.Author)
	}
	if props.Version != "" {
		metadata = append(metadata, "v"+props.Version)
	}
	if props.Runtime != "" {
		metadata = append(metadata, props.Runtime)
	}
	if props.Website != "" {
		metadata = append(metadata, props.Website)
	}
	textWidth := max(float32(0), props.Width-iconWidth-28)
	text := woxwidget.Container{Width: textWidth, Height: headerHeight, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
		woxwidget.Text{Value: props.Name, Style: woxui.TextStyle{Size: 17, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText},
		woxwidget.TextBlock{Value: props.Description, Width: textWidth, Height: 40, MaxLines: 2, Style: woxui.TextStyle{Size: 12}, LineHeight: 18, Color: props.Theme.ResultSubtitle},
		woxwidget.Text{Value: strings.Join(metadata, "  ·  "), Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ActionHeader},
	}}}
	headerChildren := make([]woxwidget.Widget, 0, 2)
	if icon != nil {
		headerChildren = append(headerChildren, icon)
	}
	headerChildren = append(headerChildren, text)
	header := woxwidget.Container{Width: props.Width, Height: headerHeight, Padding: woxwidget.Insets{Left: 14, Top: 12, Right: 14}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: headerChildren}}
	if props.Screenshot == nil || props.Height <= headerHeight+20 {
		return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: 10, Color: props.Theme.QueryBackground, Child: header}
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: 10, Color: props.Theme.QueryBackground, Child: woxwidget.Flex{
		Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{header, props.Screenshot},
	}}
}
