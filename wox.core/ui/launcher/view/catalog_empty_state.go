package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// CatalogListEmptyProps configures the centered sidebar empty state for catalog search.
type CatalogListEmptyProps struct {
	Width       float32
	Height      float32
	Title       string
	Description string
	Icon        *woxui.Image
	Window      *woxui.Window
	Theme       woxcomponent.Theme
}

// CatalogListEmptyState renders a centered icon, title, and optional subtitle for empty catalog lists.
func CatalogListEmptyState(props CatalogListEmptyProps) woxwidget.Widget {
	title := props.Title
	if title == "" {
		title = props.Description
	}
	contentWidth := min(float32(220), max(float32(0), props.Width-24))
	children := make([]woxwidget.Widget, 0, 3)
	if props.Icon != nil {
		children = append(children, woxwidget.Align{
			Width: contentWidth, Height: 28, Horizontal: 0.5, Vertical: 0.5,
			Child: woxwidget.Image{Source: props.Icon, Width: 24, Height: 24},
		})
	}
	if title != "" {
		children = append(children, catalogCenteredText(contentWidth, title, woxui.TextStyle{Size: 15, Weight: woxui.FontWeightSemibold}, props.Theme.ResultTitle))
	}
	if description := props.Description; description != "" && description != title {
		children = append(children, catalogCenteredTextBlock(props.Window, contentWidth, 17, 2, description, woxui.TextStyle{Size: 12}, props.Theme.ResultSubtitle))
	}
	content := woxwidget.Container{
		Width: contentWidth,
		Child: woxwidget.Flex{
			Axis: woxwidget.Vertical, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children,
		},
	}
	return woxwidget.Align{Width: props.Width, Height: props.Height, Horizontal: 0.5, Vertical: 0.42, Child: content}
}

func catalogCenteredText(width float32, value string, style woxui.TextStyle, color woxui.Color) woxwidget.Widget {
	lineHeight := max(style.Size*1.35, style.Size+4)
	return woxwidget.Align{
		Width: width, Height: lineHeight, Horizontal: 0.5, Vertical: 0.5,
		Child: woxwidget.Text{Value: value, Style: style, Color: color},
	}
}

func catalogCenteredTextBlock(window *woxui.Window, width, lineHeight float32, maxLines int, value string, style woxui.TextStyle, color woxui.Color) woxwidget.Widget {
	if window == nil {
		return woxwidget.Align{
			Width: width, Height: lineHeight * float32(max(1, maxLines)), Horizontal: 0.5, Vertical: 0.5,
			Child: woxwidget.TextBlock{
				Value: value, Width: width, Height: lineHeight * float32(max(1, maxLines)), MaxLines: maxLines, LineHeight: lineHeight,
				Style: style, Color: color,
			},
		}
	}
	layout := woxwidget.LayoutTextBlock(window, value, style, width, maxLines, lineHeight)
	lines := make([]woxwidget.Widget, 0, len(layout.Lines))
	for _, line := range layout.Lines {
		lines = append(lines, catalogCenteredText(width, line, style, color))
	}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: lines}
}
