package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// PreviewListItem contains the resolved data displayed by one preview list row.
type PreviewListItem struct {
	Title         string
	Subtitle      string
	Tail          string
	Icon          *woxui.Image
	FallbackColor woxui.Color
}

// PreviewListProps contains the rows rendered by a list preview.
type PreviewListProps struct {
	Width  float32
	Height float32
	Items  []PreviewListItem
	Theme  woxcomponent.Theme
}

// PreviewList builds a compact list preview.
func PreviewList(props PreviewListProps) woxwidget.Widget {
	if len(props.Items) == 0 {
		return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: 10, Color: props.Theme.QueryBackground, Padding: woxwidget.UniformInsets(14), Child: woxwidget.Text{Value: "No items", Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle}}
	}
	const rowHeight = float32(54)
	visibleCount := min(len(props.Items), max(1, int(props.Height/rowHeight)))
	rows := make([]woxwidget.Widget, 0, visibleCount)
	for index := 0; index < visibleCount; index++ {
		item := props.Items[index]
		var icon woxwidget.Widget = woxwidget.Container{Width: 30, Height: 30, Radius: 7, Color: item.FallbackColor}
		if item.Icon != nil {
			icon = woxwidget.Image{Source: item.Icon, Width: 30, Height: 30}
		}
		tailWidth := float32(0)
		var tail woxwidget.Widget
		if item.Tail != "" {
			tailWidth = 78
			tail = woxwidget.Container{Width: tailWidth, Height: 30, Padding: woxwidget.Insets{Top: 7}, Child: woxwidget.Text{Value: item.Tail, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle}}
		}
		labelWidth := max(float32(40), props.Width-30-tailWidth-58)
		rows = append(rows, woxwidget.Container{Width: max(float32(0), props.Width-20), Height: rowHeight, Padding: woxwidget.Insets{Left: 10, Top: 10, Right: 10, Bottom: 8}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
			icon,
			woxwidget.Container{Width: labelWidth, Height: 36, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 2, Children: []woxwidget.Widget{
				woxwidget.Text{Value: item.Title, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.PreviewText},
				woxwidget.Text{Value: item.Subtitle, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle},
			}}},
			tail,
		}}})
	}
	return woxwidget.Container{Width: props.Width, Height: props.Height, Padding: woxwidget.Insets{Top: 4}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}
}
