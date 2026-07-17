package preview

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// editorPreviewShellProps contains the shared scrolling, error, and save-footer state for preview editors.
type editorPreviewShellProps struct {
	Width             float32
	Height            float32
	Padding           woxwidget.Insets
	Theme             woxcomponent.Theme
	BeforeBody        []woxwidget.Widget
	BeforeBodyHeight  float32
	MinimumBodyHeight float32
	Rows              []woxwidget.Widget
	RowsHeight        float32
	EmptyMessage      string
	ScrollID          string
	KeepVisible       *woxwidget.ScrollRange
	Error             string
	ShowError         bool
	SaveButton        woxcomponent.ButtonProps
}

// editorPreviewShell builds the common viewport, error line, and trailing save action.
func editorPreviewShell(props editorPreviewShellProps) woxwidget.Widget {
	const footerHeight = float32(48)
	innerWidth := max(float32(0), props.Width-props.Padding.Left-props.Padding.Right)
	innerHeight := max(float32(0), props.Height-props.Padding.Top-props.Padding.Bottom)
	errorHeight := float32(0)
	if props.ShowError {
		errorHeight = 30
	}
	bodyHeight := max(props.MinimumBodyHeight, innerHeight-props.BeforeBodyHeight-footerHeight-errorHeight)
	var body woxwidget.Widget
	if len(props.Rows) == 0 && props.EmptyMessage != "" {
		body = woxwidget.Container{Width: innerWidth, Height: bodyHeight, Padding: woxwidget.Insets{Top: 12}, Child: woxwidget.Text{
			Value: props.EmptyMessage, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle,
		}}
	} else {
		body = woxwidget.ScrollView{
			Key: woxwidget.Key(props.ScrollID), ID: props.ScrollID, Width: innerWidth, Height: bodyHeight,
			ContentHeight: max(bodyHeight, props.RowsHeight), KeepVisible: props.KeepVisible,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: props.Rows},
		}
	}
	button := woxcomponent.WoxButton(props.SaveButton)
	footer := woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
		woxwidget.Painter{Width: max(float32(0), innerWidth-props.SaveButton.Width), Height: footerHeight},
		button,
	}}
	children := make([]woxwidget.Widget, 0, len(props.BeforeBody)+3)
	children = append(children, props.BeforeBody...)
	children = append(children, body)
	if props.ShowError {
		children = append(children, woxwidget.Container{Width: innerWidth, Height: errorHeight, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{
			Value: props.Error, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ErrorText,
		}})
	}
	children = append(children, footer)
	return woxwidget.Container{
		Width: props.Width, Height: props.Height, Padding: props.Padding,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children},
	}
}
