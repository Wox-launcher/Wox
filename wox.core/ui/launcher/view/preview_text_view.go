package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ScrollablePreviewTextProps contains a laid-out text preview and its scroll action.
type ScrollablePreviewTextProps struct {
	ID       string
	Value    string
	Color    woxui.Color
	Width    float32
	Height   float32
	Layout   woxwidget.TextBlockLayout
	Offset   float32
	OnScroll func(float32, float32)
}

// ScrollablePreviewText builds a scrollable generic text preview.
func ScrollablePreviewText(props ScrollablePreviewTextProps) woxwidget.Widget {
	innerWidth := max(float32(0), props.Width-48)
	innerHeight := max(float32(0), props.Height-48)
	contentHeight := max(innerHeight, props.Layout.Size.Height)
	maxOffset := max(float32(0), contentHeight-innerHeight)
	offset := min(max(float32(0), props.Offset), maxOffset)
	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Padding: woxwidget.UniformInsets(24),
		Child: woxwidget.Gesture{
			ID: "preview-scroll-" + props.ID,
			OnScroll: func(delta woxui.Point) {
				if props.OnScroll != nil {
					props.OnScroll(-delta.Y, maxOffset)
				}
			},
			Child: woxwidget.ScrollView{
				Width: innerWidth, Height: innerHeight, ContentHeight: contentHeight, Offset: offset,
				Child: woxwidget.TextBlock{Value: props.Value, Width: innerWidth, Height: contentHeight, Style: woxui.TextStyle{Size: 15}, LineHeight: 23, Color: props.Color, Layout: &props.Layout},
			},
		},
	}
}

// TextPreviewProps contains the centered quote layout.
type TextPreviewProps struct {
	Value  string
	Width  float32
	Height float32
	Layout woxwidget.TextBlockLayout
	Theme  woxcomponent.Theme
	Window *woxui.Window
}

// TextPreviewFits reports whether the centered quote treatment can display every line.
func TextPreviewFits(layout woxwidget.TextBlockLayout, width, height float32) bool {
	const horizontalPadding = float32(44)
	const verticalPadding = float32(62)
	return width-horizontalPadding*2 > 0 && layout.Size.Height <= max(float32(0), height-verticalPadding*2)
}

// TextPreview applies the centered quote treatment when the complete text fits safely.
func TextPreview(props TextPreviewProps) woxwidget.Widget {
	const verticalPadding = float32(62)
	if !TextPreviewFits(props.Layout, props.Width, props.Height) {
		return woxwidget.Container{Width: props.Width, Height: props.Height}
	}
	style := woxui.TextStyle{Size: 17}
	lineHeight := float32(25)
	textTop := max(verticalPadding, (props.Height-props.Layout.Size.Height)*0.5)
	bodyColor := previewColorWithOpacity(props.Theme.PreviewText, 0.86)
	quoteColor := previewColorWithOpacity(props.Theme.PreviewText, 0.16)
	return woxwidget.Painter{Width: props.Width, Height: props.Height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		quoteStyle := woxui.TextStyle{Size: 72, Weight: woxui.FontWeightSemibold}
		displayList.DrawText("“", woxui.Rect{X: bounds.X + 22, Y: bounds.Y + 12, Width: 86, Height: 78}, quoteStyle, quoteColor)
		closingMetrics, _ := props.Window.MeasureText("”", quoteStyle)
		displayList.DrawText("”", woxui.Rect{X: bounds.X + bounds.Width - 22 - closingMetrics.Size.Width, Y: bounds.Y + bounds.Height - 76, Width: closingMetrics.Size.Width, Height: 78}, quoteStyle, quoteColor)
		for index, line := range props.Layout.Lines {
			metrics, _ := props.Window.MeasureText(line, style)
			left := bounds.X + (bounds.Width-metrics.Size.Width)*0.5
			top := bounds.Y + textTop + float32(index)*lineHeight
			displayList.DrawText(line, woxui.Rect{X: left, Y: top, Width: metrics.Size.Width, Height: lineHeight}, style, bodyColor)
		}
	}}
}
