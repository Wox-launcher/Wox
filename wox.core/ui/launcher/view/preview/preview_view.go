package preview

import (
	"fmt"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	previewSurfaceRadius      float32 = 8
	previewSurfaceBorderWidth float32 = 1
)

// PreviewProps contains the content and metadata rendered by the generic preview shell.
type PreviewProps struct {
	Width      float32
	Height     float32
	Tags       []PreviewTag
	Body       woxwidget.Widget
	Theme      woxcomponent.Theme
	Window     *woxui.Window
	OnTagHover func(bool, string, woxui.Rect)
}

// PreviewTag keeps compact metadata text and its explanatory tooltip together.
type PreviewTag struct {
	Label   string
	Tooltip string
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
		children = append(children, woxwidget.StackChild{Top: surfaceHeight + 10, Child: PreviewTags(props.Tags, props.Theme, props.Window, layout.InnerWidth, props.OnTagHover)})
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
		Width: width, Height: height, Radius: previewSurfaceRadius, Color: previewColorWithOpacity(theme.PreviewText, 0.035),
		BorderColor: previewColorWithOpacity(theme.PreviewSplit, 0.45), BorderWidth: previewSurfaceBorderWidth, Padding: woxwidget.UniformInsets(previewSurfaceBorderWidth),
		Child: woxwidget.Clip{Width: contentWidth, Height: contentHeight, Child: body},
	}
}

// PreviewTags builds the metadata pills shared by real and theme-editor previews.
func PreviewTags(tags []PreviewTag, theme woxcomponent.Theme, window *woxui.Window, width float32, onHover func(bool, string, woxui.Rect)) woxwidget.Widget {
	children := make([]woxwidget.Widget, 0, len(tags))
	contentWidth := float32(0)
	for index, tag := range tags {
		label := strings.TrimSpace(tag.Label)
		if strings.TrimSpace(label) == "" {
			continue
		}
		style := woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}
		metrics, _ := window.MeasureText(label, style)
		chipWidth := min(max(float32(36), metrics.Size.Width+18), min(float32(220), max(float32(36), width)))
		if len(children) > 0 {
			contentWidth += 8
		}
		pill := woxwidget.Container{
			Width: chipWidth, Height: 26, Radius: 8, Color: previewColorWithOpacity(theme.PreviewText, 0.035),
			BorderColor: previewColorWithOpacity(theme.PreviewPropertyTitle, 0.48), BorderWidth: 1,
			Padding: woxwidget.Insets{Left: 9, Top: 6, Right: 9, Bottom: 5},
			Child:   woxwidget.Text{Value: label, Style: style, Color: previewColorWithOpacity(theme.PreviewPropertyContent, 0.9)},
		}
		tooltip := strings.TrimSpace(tag.Tooltip)
		if tooltip == "" {
			tooltip = label
		}
		if onHover != nil {
			id := fmt.Sprintf("preview-tag-%d", index)
			pill = woxwidget.Container{Width: chipWidth, Height: 26, Child: woxwidget.Semantics{
				Key: woxwidget.Key(id), AutomationID: id, Role: woxui.AccessibilityRoleText, Label: label, Description: tooltip,
				Child: woxwidget.Gesture{
					ID: id, OnHoverAt: func(inside bool, bounds woxui.Rect) { onHover(inside, tooltip, bounds) }, Child: pill,
				},
			}}
		}
		children = append(children, pill)
		contentWidth += chipWidth
	}
	// Keep a stable key so this strip is a retained ScrollView, not a clip-only
	// primitive. Map the vertical mouse wheel because this footer is not nested
	// inside another scroller and Windows wheels rarely emit a horizontal delta.
	return woxwidget.ScrollView{
		Key: "preview-tags", Width: width, Height: 26, ContentWidth: max(width, contentWidth),
		Horizontal: true, MapVerticalWheel: true,
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: children},
	}
}

func previewColorWithOpacity(color woxui.Color, opacity float32) woxui.Color {
	opacity = min(max(float32(0), opacity), float32(1))
	color.A = uint8(opacity*255 + 0.5)
	return color
}
