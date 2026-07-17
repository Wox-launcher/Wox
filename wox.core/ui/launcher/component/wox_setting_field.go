package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// SettingFieldProps describes one label, description, and control row.
type SettingFieldProps struct {
	Label               string
	Description         string
	Width               float32
	Height              float32
	LabelWidth          float32
	Gap                 float32
	Radius              float32
	Background          woxui.Color
	Padding             woxwidget.Insets
	DescriptionMaxLines int
	Child               woxwidget.Widget
	Theme               Theme
}

// WoxSettingField builds the shared horizontal settings field layout.
func WoxSettingField(props SettingFieldProps) woxwidget.Widget {
	height := props.Height
	if height <= 0 {
		height = 66
	}
	gap := props.Gap
	if gap <= 0 {
		gap = 20
	}
	labelHeight := max(float32(0), height-props.Padding.Top-props.Padding.Bottom)
	descriptionHeight := float32(18)
	var description woxwidget.Widget = woxwidget.Text{Value: props.Description, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle}
	if props.DescriptionMaxLines > 1 {
		descriptionHeight = float32(props.DescriptionMaxLines * 16)
		description = woxwidget.TextBlock{
			Value: props.Description, Width: props.LabelWidth, Height: descriptionHeight, MaxLines: props.DescriptionMaxLines,
			Style: woxui.TextStyle{Size: 11}, LineHeight: 16, Color: props.Theme.ResultSubtitle,
		}
	}
	label := woxwidget.Container{Width: props.LabelWidth, Height: labelHeight, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: []woxwidget.Widget{
		woxwidget.Text{Value: props.Label, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
		description,
	}}}
	return woxwidget.Container{Width: props.Width, Height: height, Radius: props.Radius, Color: props.Background, Padding: props.Padding, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: gap, Children: []woxwidget.Widget{label, props.Child},
	}}
}
