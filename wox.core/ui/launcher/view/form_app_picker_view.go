package view

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const formAppPickerRowHeight = float32(58)

// FormAppCandidate is the display data required for one application picker row.
type FormAppCandidate struct {
	Name          string
	Detail        string
	Icon          *woxui.Image
	FallbackColor woxui.Color
}

// FormAppPickerProps contains the immutable state and actions rendered by the application picker.
type FormAppPickerProps struct {
	Width      float32
	Height     float32
	Theme      woxcomponent.Theme
	Candidates []FormAppCandidate
	Selected   int
	OnChoose   func(int)
	OnCancel   func()
}

// FormAppPickerView builds the reusable application picker list.
func FormAppPickerView(props FormAppPickerProps) woxwidget.Widget {
	footerHeight := float32(54)
	viewportHeight := max(float32(58), props.Height-footerHeight)
	rows := make([]woxwidget.Widget, 0, len(props.Candidates))
	for index, candidate := range props.Candidates {
		index := index
		foreground := props.Theme.ActionText
		if index == props.Selected {
			foreground = props.Theme.SelectedTitle
		}
		var icon woxwidget.Widget = woxwidget.Container{Width: 34, Height: 34, Radius: 7, Color: candidate.FallbackColor}
		if candidate.Icon != nil {
			icon = woxwidget.Image{Source: candidate.Icon, Width: 34, Height: 34}
		}
		rows = append(rows, woxcomponent.WoxListItem(woxcomponent.ListItemProps{
			ID: fmt.Sprintf("form-table-app-%d", index), Label: candidate.Name, Width: props.Width, Height: formAppPickerRowHeight,
			Selected: index == props.Selected, Padding: woxwidget.Insets{Left: 10, Top: 10, Right: 10}, Theme: props.Theme,
			OnTap: func() {
				if props.OnChoose != nil {
					props.OnChoose(index)
				}
			},
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
				icon,
				woxwidget.Container{Width: max(float32(0), props.Width-66), Height: 40, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
					woxwidget.Text{Value: candidate.Name, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: foreground},
					woxwidget.Text{Value: compactViewText(candidate.Detail, 96), Style: woxui.TextStyle{Size: 9}, Color: props.Theme.ResultSubtitle},
				}}},
			}},
		}))
	}
	var list woxwidget.Widget
	if len(rows) == 0 {
		list = woxwidget.Container{Width: props.Width, Height: viewportHeight, Radius: 8, Color: props.Theme.QueryBackground, Padding: woxwidget.Insets{Left: 16, Top: 18}, Child: woxwidget.Text{
			Value: "No application candidates are available on this platform.", Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle,
		}}
	} else {
		var keepVisible *woxwidget.ScrollRange
		if props.Selected >= 0 && props.Selected < len(rows) {
			start := float32(props.Selected) * formAppPickerRowHeight
			keepVisible = &woxwidget.ScrollRange{Start: start, End: start + formAppPickerRowHeight}
		}
		list = woxwidget.ScrollView{
			Key: "form-table-app-scroll", ID: "form-table-app-scroll", Width: props.Width, Height: viewportHeight,
			ContentHeight: max(viewportHeight, float32(len(rows))*formAppPickerRowHeight), KeepVisible: keepVisible,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}
	}
	footer := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Container{Width: max(float32(0), props.Width-112), Height: 42, Padding: woxwidget.Insets{Top: 13}, Child: woxwidget.Text{
			Value: "Select application · ↑↓ move · Enter choose · Esc cancel", Style: woxui.TextStyle{Size: 10}, Color: props.Theme.ResultSubtitle,
		}},
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "form-table-app-cancel", Label: "Cancel", Width: 104, OnTap: props.OnCancel, Theme: props.Theme}),
	}}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
		list,
		woxwidget.Container{Width: props.Width, Height: footerHeight, Padding: woxwidget.Insets{Top: 8}, Child: footer},
	}}
}
