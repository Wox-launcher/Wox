package component

import (
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// SectionHeaderProps describes a divider and label between settings groups.
type SectionHeaderProps struct {
	Label       string
	Width       float32
	Action      woxwidget.Widget
	ActionWidth float32
	Theme       Theme
}

// WoxSectionHeader builds the shared settings section divider.
func WoxSectionHeader(props SectionHeaderProps) woxwidget.Widget {
	action := props.Action
	actionWidth := props.ActionWidth
	if action == nil {
		action = woxwidget.Painter{}
		actionWidth = 0
	}
	return woxwidget.Container{Width: props.Width, Height: 43, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: props.Width, Height: 1, Color: props.Theme.PreviewSplit},
		woxwidget.Container{Width: props.Width, Height: 42, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: max(float32(0), props.Width-actionWidth), Height: 42, Padding: woxwidget.Insets{Top: 14}, Child: woxwidget.Text{
				Value: strings.ToUpper(props.Label), Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle,
			}},
			action,
		}}},
	}}}
}
