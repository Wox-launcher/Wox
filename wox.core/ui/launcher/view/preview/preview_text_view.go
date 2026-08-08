package preview

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// ScrollablePreviewTextProps contains a laid-out text preview and its scroll action.
type ScrollablePreviewTextProps struct {
	ID            string
	Value         string
	Color         woxui.Color
	Width         float32
	Height        float32
	FontSize      float32
	LineHeight    float32
	Layout        woxwidget.TextBlockLayout
	InitialOffset float32
}

// ScrollablePreviewTextHorizontalPadding keeps adapter text measurement aligned with the viewport.
const ScrollablePreviewTextHorizontalPadding = float32(14)

// ScrollablePreviewText builds a scrollable generic text preview.
func ScrollablePreviewText(props ScrollablePreviewTextProps) woxwidget.Widget {
	const verticalPadding = float32(24)
	innerWidth := max(float32(0), props.Width-ScrollablePreviewTextHorizontalPadding*2)
	innerHeight := max(float32(0), props.Height-verticalPadding*2)
	contentHeight := max(innerHeight, props.Layout.Size.Height)
	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: ScrollablePreviewTextHorizontalPadding, Top: verticalPadding, Right: ScrollablePreviewTextHorizontalPadding, Bottom: verticalPadding},
		Child: woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: woxwidget.Key("preview-scroll-" + props.ID), Offset: props.InitialOffset, Width: innerWidth, Height: innerHeight, ContentHeight: contentHeight,
			Content:    woxwidget.TextBlock{Value: props.Value, Width: innerWidth, Height: contentHeight, Style: woxui.TextStyle{Size: props.FontSize}, LineHeight: props.LineHeight, Color: props.Color, Layout: &props.Layout},
			ThumbColor: props.Color,
		}),
	}
}

// TextPreviewProps contains the centered quote layout.
type TextPreviewProps struct {
	Value      string
	Width      float32
	Height     float32
	FontSize   float32
	LineHeight float32
	Layout     woxwidget.TextBlockLayout
	Theme      woxcomponent.Theme
	Window     *woxui.Window
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
	style := woxui.TextStyle{Size: props.FontSize}
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
			top := bounds.Y + textTop + float32(index)*props.LineHeight
			displayList.DrawText(line, woxui.Rect{X: left, Y: top, Width: metrics.Size.Width, Height: props.LineHeight}, style, bodyColor)
		}
	}}
}
