package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// PageHeaderProps describes the standard title and description block for a Wox page.
type PageHeaderProps struct {
	Title       string
	Description string
	Width       float32
	Height      float32
	TitleSize   float32
	Gap         float32
	Theme       Theme
}

// WoxPageHeader builds the shared heading used by launcher management pages.
func WoxPageHeader(props PageHeaderProps) woxwidget.Widget {
	height := props.Height
	if height <= 0 {
		height = 72
	}
	titleSize := props.TitleSize
	if titleSize <= 0 {
		titleSize = 22
	}
	gap := props.Gap
	if gap <= 0 {
		gap = 6
	}
	return woxwidget.Container{Width: props.Width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: gap, Children: []woxwidget.Widget{
		woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: titleSize, Weight: woxui.FontWeightSemibold}, Color: props.Theme.QueryText},
		woxwidget.Text{Value: props.Description, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle},
	}}}
}
