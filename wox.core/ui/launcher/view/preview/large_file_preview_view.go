package preview

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// LargeFilePreviewProperty is one metadata row shown before a deferred file preview loads.
type LargeFilePreviewProperty struct {
	Label string
	Value string
}

// LargeFilePreviewProps contains the Flutter-style large-file gate shown in the preview pane.
type LargeFilePreviewProps struct {
	Width      float32
	Height     float32
	Theme      woxcomponent.Theme
	Title      string
	Message    string
	Properties []LargeFilePreviewProperty
	Action     string
	OnLoad     func()
}

const largeFilePreviewPropertyLabelWidth = float32(88)

// LargeFilePreviewView shows file details first so large documents are not opened automatically.
func LargeFilePreviewView(props LargeFilePreviewProps) woxwidget.Widget {
	innerWidth := max(float32(0), props.Width-36)
	children := make([]woxwidget.Widget, 0, 3)
	if props.Title != "" || props.Action != "" {
		children = append(children, largeFilePreviewHeader(props, innerWidth))
	}
	if props.Message != "" {
		children = append(children, woxwidget.TextBlock{Value: props.Message, Width: innerWidth, Style: woxui.TextStyle{Size: 13}, LineHeight: 18, Color: props.Theme.ResultSubtitle})
	}
	if len(props.Properties) > 0 {
		rows := make([]woxwidget.Widget, 0, len(props.Properties))
		valueWidth := max(float32(0), innerWidth-largeFilePreviewPropertyLabelWidth-10)
		for _, property := range props.Properties {
			rows = append(rows, woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisStart, Children: []woxwidget.Widget{
				woxwidget.Container{Width: largeFilePreviewPropertyLabelWidth, Child: woxwidget.Text{Value: property.Label, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.PreviewPropertyTitle}},
				woxwidget.Expanded{Child: woxwidget.TextBlock{Value: property.Value, Width: valueWidth, Style: woxui.TextStyle{Size: 12}, LineHeight: 16, Color: props.Theme.PreviewPropertyContent}},
			}})
		}
		children = append(children, woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: rows})
	}
	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Left: 18, Top: 16, Right: 18, Bottom: 16},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 12, Children: children},
	}
}

// largeFilePreviewHeader keeps the title on the left and the load action trailing on the same row.
func largeFilePreviewHeader(props LargeFilePreviewProps, width float32) woxwidget.Widget {
	title := woxwidget.Text{Value: props.Title, Style: woxui.TextStyle{Size: 15, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText}
	if props.Action == "" {
		return title
	}
	button := woxcomponent.WoxButton(woxcomponent.ButtonProps{
		ID: "file-preview-load", Label: props.Action, IntrinsicWidth: true, Variant: woxcomponent.ButtonOutline, Theme: props.Theme, OnTap: props.OnLoad,
	})
	if props.Title == "" {
		return woxwidget.Align{Width: width, Horizontal: 1, Child: button}
	}
	// Pin the header to the button height. A width-only Container inherits the
	// preview's available height and hides the file details below the title.
	return woxwidget.Container{Width: width, Height: 32, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxwidget.Expanded{Child: woxwidget.Align{Vertical: 0.5, Child: title}},
		button,
	}}}
}
