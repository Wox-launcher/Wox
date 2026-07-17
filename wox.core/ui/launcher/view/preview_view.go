package view

import (
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// PreviewProps contains the content and metadata rendered by the generic preview shell.
type PreviewProps struct {
	Width  float32
	Height float32
	Tags   []string
	Body   woxwidget.Widget
	Theme  woxcomponent.Theme
	Window *woxui.Window
}

// PreviewLayout contains the body dimensions shared by the adapter and preview shell.
type PreviewLayout struct {
	BodyWidth   float32
	BodyHeight  float32
	InnerWidth  float32
	InnerHeight float32
}

// ResolvePreviewLayout calculates the body size after optional metadata tags are reserved.
func ResolvePreviewLayout(width, height float32, hasTags bool) PreviewLayout {
	innerWidth := max(float32(0), width-26)
	innerHeight := max(float32(0), height-22)
	bodyHeight := innerHeight
	if hasTags {
		bodyHeight = max(float32(0), innerHeight-36)
	}
	return PreviewLayout{BodyWidth: max(float32(0), innerWidth-2), BodyHeight: max(float32(0), bodyHeight-2), InnerWidth: innerWidth, InnerHeight: innerHeight}
}

// PreviewView builds the generic preview surface and its optional metadata tags.
func PreviewView(props PreviewProps) woxwidget.Widget {
	layout := ResolvePreviewLayout(props.Width, props.Height, len(props.Tags) > 0)
	surfaceHeight := layout.BodyHeight + 2
	children := []woxwidget.StackChild{{Child: previewSurface(props.Body, props.Theme, layout.InnerWidth, surfaceHeight)}}
	if len(props.Tags) > 0 {
		children = append(children, woxwidget.StackChild{Top: surfaceHeight + 10, Child: previewTags(props.Tags, props.Theme, props.Window, layout.InnerWidth)})
	}
	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 14, Top: 12, Right: 12, Bottom: 10},
		Child: woxwidget.Stack{Width: layout.InnerWidth, Height: layout.InnerHeight, Children: children},
	}
}

func previewSurface(body woxwidget.Widget, theme woxcomponent.Theme, width, height float32) woxwidget.Widget {
	contentWidth := max(float32(0), width-2)
	contentHeight := max(float32(0), height-2)
	return woxwidget.Container{
		Width: width, Height: height, Radius: 8, Color: previewColorWithOpacity(theme.PreviewSplit, 0.45), Padding: woxwidget.UniformInsets(1),
		Child: woxwidget.Container{
			Width: contentWidth, Height: contentHeight, Radius: 7, Color: previewOpaqueOverlay(theme.Background, theme.PreviewText, 0.035),
			Child: woxwidget.Clip{Width: contentWidth, Height: contentHeight, Child: body},
		},
	}
}

func previewTags(tags []string, theme woxcomponent.Theme, window *woxui.Window, width float32) woxwidget.Widget {
	children := make([]woxwidget.Widget, 0, len(tags))
	used := float32(0)
	for _, label := range tags {
		if strings.TrimSpace(label) == "" {
			continue
		}
		style := woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}
		metrics, _ := window.MeasureText(label, style)
		chipWidth := min(max(float32(36), metrics.Size.Width+18), min(float32(220), max(float32(36), width)))
		if used > 0 && used+8+chipWidth > width {
			break
		}
		children = append(children, woxwidget.Container{
			Width: chipWidth, Height: 26, Radius: 8, Color: previewColorWithOpacity(theme.PreviewPropertyTitle, 0.48), Padding: woxwidget.UniformInsets(1),
			Child: woxwidget.Container{
				Width: chipWidth - 2, Height: 24, Radius: 7, Color: previewOpaqueOverlay(theme.Background, theme.PreviewText, 0.035),
				Padding: woxwidget.Insets{Left: 8, Top: 5, Right: 8, Bottom: 4},
				Child:   woxwidget.Text{Value: label, Style: style, Color: previewColorWithOpacity(theme.PreviewPropertyContent, 0.9)},
			},
		})
		used += chipWidth + 8
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: children}
}

func previewColorWithOpacity(color woxui.Color, opacity float32) woxui.Color {
	opacity = min(max(float32(0), opacity), float32(1))
	color.A = uint8(opacity*255 + 0.5)
	return color
}

// previewOpaqueOverlay prevents a translucent border from tinting the nested surface.
func previewOpaqueOverlay(background, foreground woxui.Color, opacity float32) woxui.Color {
	opacity = min(max(float32(0), opacity), float32(1))
	blend := func(base, overlay uint8) uint8 {
		return uint8(float32(base) + (float32(overlay)-float32(base))*opacity + 0.5)
	}
	return woxui.Color{R: blend(background.R, foreground.R), G: blend(background.G, foreground.G), B: blend(background.B, foreground.B), A: 255}
}
