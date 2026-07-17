package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// PageHeaderHeight is the fixed height shared by settings page headers.
const PageHeaderHeight = float32(72)

// PageHeaderProps describes the standard title and description block for a Wox page.
type PageHeaderProps struct {
	Title       string
	Description string
	Width       float32
	Theme       Theme
}

// WoxPageHeader builds the shared title and description block used by settings pages.
func WoxPageHeader(props PageHeaderProps) woxwidget.Widget {
	return woxwidget.Container{Width: props.Width, Height: PageHeaderHeight, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
		woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 22, Weight: woxui.FontWeightSemibold}, Color: props.Theme.QueryText},
		woxwidget.Text{Value: props.Description, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle},
	}}}
}
