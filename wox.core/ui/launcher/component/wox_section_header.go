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
	title := woxwidget.Container{Height: 42, Padding: woxwidget.Insets{Top: 14}, Child: woxwidget.Text{
		Value: strings.ToUpper(props.Label), Style: woxui.TextStyle{Size: SettingsSectionTitleFontSize, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle,
	}}
	children := []woxwidget.Widget{woxwidget.Expanded{Child: title}}
	if props.Action != nil {
		action := props.Action
		if props.ActionWidth > 0 {
			action = woxwidget.Constrained{MinWidth: props.ActionWidth, MaxWidth: props.ActionWidth, FillWidth: true, Child: action}
		}
		children = append(children, action)
	}
	return woxwidget.Container{Width: props.Width, Height: 43, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		woxwidget.Container{Width: props.Width, Height: 1, Color: withAlpha(props.Theme.ToolbarText, 26)},
		woxwidget.Container{Width: props.Width, Height: 42, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: children}},
	}}}
}
