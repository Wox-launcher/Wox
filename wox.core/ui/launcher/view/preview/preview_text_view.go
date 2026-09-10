package preview

import (
	"strings"

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
	InitialOffset float32
	Window        *woxui.Window
	Theme         woxcomponent.Theme
}

// ScrollablePreviewTextHorizontalPadding keeps adapter text measurement aligned with the viewport.
const ScrollablePreviewTextHorizontalPadding = float32(14)

// ScrollablePreviewText builds a scrollable generic text preview.
func ScrollablePreviewText(props ScrollablePreviewTextProps) woxwidget.Widget {
	const verticalPadding = float32(24)
	innerWidth := max(float32(0), props.Width-ScrollablePreviewTextHorizontalPadding*2)
	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: ScrollablePreviewTextHorizontalPadding, Top: verticalPadding, Right: ScrollablePreviewTextHorizontalPadding, Bottom: verticalPadding},
		Child: woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
			Key: woxwidget.Key("preview-scroll-" + props.ID), Offset: props.InitialOffset, FillWidth: true, FillHeight: true,
			Content:    selectablePreviewText(previewTextFieldID(props.ID, "text"), props.Value, innerWidth, props.FontSize, props.LineHeight, props.Color, props.Window, props.Theme),
			ThumbColor: props.Color,
		}),
	}
}

// TextPreviewProps contains the centered quote layout.
type TextPreviewProps struct {
	ID         string
	Value      string
	Width      float32
	Height     float32
	FontSize   float32
	LineHeight float32
	Theme      woxcomponent.Theme
	Window     *woxui.Window
}

const (
	textPreviewHorizontalPadding = float32(44)
	textPreviewVerticalPadding   = float32(62)
)

// TextPreviewFits reports whether the centered quote treatment can display every line.
func TextPreviewFits(value string, window *woxui.Window, style woxui.TextStyle, width, height, lineHeight float32) bool {
	innerWidth := max(float32(0), width-textPreviewHorizontalPadding*2)
	if innerWidth <= 0 {
		return false
	}
	lines := woxcomponent.TextFieldVisualLineCount(value, window, style, innerWidth, nil)
	return float32(max(1, lines))*lineHeight <= max(float32(0), height-textPreviewVerticalPadding*2)
}

// TextPreview applies the centered quote treatment when the complete text fits safely.
func TextPreview(props TextPreviewProps) woxwidget.Widget {
	style := woxui.TextStyle{Size: props.FontSize}
	if !TextPreviewFits(props.Value, props.Window, style, props.Width, props.Height, props.LineHeight) {
		return woxwidget.Container{Width: props.Width, Height: props.Height}
	}
	innerWidth := max(float32(0), props.Width-textPreviewHorizontalPadding*2)
	fieldWidth := quotePreviewTextWidth(props.Value, props.Window, style, innerWidth)
	bodyColor := previewColorWithOpacity(props.Theme.PreviewText, 0.86)
	return woxwidget.Stack{
		Width: props.Width, Height: props.Height,
		Children: []woxwidget.StackChild{
			{Child: quotePreviewMarks(props)},
			{Child: woxwidget.Align{
				Width: props.Width, Height: props.Height, Horizontal: 0.5, Vertical: 0.5,
				Child: selectablePreviewText(previewTextFieldID(props.ID, "quote"), props.Value, fieldWidth, props.FontSize, props.LineHeight, bodyColor, props.Window, props.Theme),
			}},
		},
	}
}

// selectablePreviewText uses a read-only text field so users can drag-select and copy preview text.
func selectablePreviewText(id, value string, width, fontSize, lineHeight float32, color woxui.Color, window *woxui.Window, theme woxcomponent.Theme) woxwidget.Widget {
	style := woxui.TextStyle{Size: fontSize}
	lines := woxcomponent.TextFieldVisualLineCount(value, window, style, width, nil)
	height := max(lineHeight, float32(max(1, lines))*lineHeight+1)
	return woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
		ID: id, Label: value, Width: width, Height: height, Padding: woxwidget.Insets{Bottom: 1},
		Transparent: true, DisableHover: true, Style: style, LineHeight: lineHeight,
		TextColor: color, Value: value, ReadOnly: true, MaxLines: max(8, lines+4),
		Window: window, Theme: theme,
	})
}

func previewTextFieldID(id, kind string) string {
	if id = strings.TrimSpace(id); id != "" {
		return "preview-" + kind + "-" + id
	}
	return "preview-" + kind
}

func quotePreviewTextWidth(value string, window *woxui.Window, style woxui.TextStyle, maxWidth float32) float32 {
	if window == nil || maxWidth <= 0 {
		return maxWidth
	}
	widest := float32(0)
	for _, line := range strings.Split(value, "\n") {
		if line == "" {
			continue
		}
		metrics, _ := window.MeasureText(line, style)
		if metrics.Size.Width > widest {
			widest = metrics.Size.Width
		}
	}
	if widest <= 0 {
		return maxWidth
	}
	return min(maxWidth, widest)
}

func quotePreviewMarks(props TextPreviewProps) woxwidget.Widget {
	quoteColor := previewColorWithOpacity(props.Theme.PreviewText, 0.16)
	return woxwidget.Painter{Width: props.Width, Height: props.Height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		quoteStyle := woxui.TextStyle{Size: 72, Weight: woxui.FontWeightSemibold}
		displayList.DrawText("“", woxui.Rect{X: bounds.X + 22, Y: bounds.Y + 12, Width: 86, Height: 78}, quoteStyle, quoteColor)
		closingMetrics, _ := props.Window.MeasureText("”", quoteStyle)
		displayList.DrawText("”", woxui.Rect{X: bounds.X + bounds.Width - 22 - closingMetrics.Size.Width, Y: bounds.Y + bounds.Height - 76, Width: closingMetrics.Size.Width, Height: 78}, quoteStyle, quoteColor)
	}}
}
