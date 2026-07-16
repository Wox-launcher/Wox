package launcher

import (
	"fmt"
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func (a *App) buildFormTableAppPicker(snapshot *formTableAppPickerSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	footerHeight := float32(54)
	viewportHeight := max(float32(58), height-footerHeight)
	a.setFormTableAppPickerViewport(viewportHeight)
	rows := make([]woxwidget.Widget, 0, len(snapshot.candidates))
	for index, candidate := range snapshot.candidates {
		index := index
		background := palette.queryBackground
		foreground := palette.actionText
		if index == snapshot.selected {
			background = palette.selectedBackground
			foreground = palette.selectedTitle
		}
		var icon woxwidget.Widget = woxwidget.Container{Width: 34, Height: 34, Radius: 7, Color: resultColors[index%len(resultColors)]}
		if image := a.imageFor(candidate.Icon); image != nil {
			icon = woxwidget.Image{Source: image, Width: 34, Height: 34}
		}
		detail := strings.TrimSpace(candidate.Path)
		if detail == "" {
			detail = candidate.Identity
		}
		rows = append(rows, woxwidget.Gesture{ID: fmt.Sprintf("form-table-app-%d", index), OnTap: func() { a.chooseFormTableAppCandidate(index) }, Child: woxwidget.Container{
			Width: width, Height: formTableAppPickerRowHeight, Radius: 7, Color: background, Padding: woxwidget.Insets{Left: 10, Top: 10, Right: 10},
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
				icon,
				woxwidget.Container{Width: max(float32(0), width-66), Height: 40, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: []woxwidget.Widget{
					woxwidget.Text{Value: candidate.Name, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: foreground},
					woxwidget.Text{Value: compactFormTableText(detail, 96), Style: woxui.TextStyle{Size: 9}, Color: palette.actionHeader},
				}}},
			}},
		}})
	}
	var list woxwidget.Widget
	if len(rows) == 0 {
		list = woxwidget.Container{Width: width, Height: viewportHeight, Radius: 8, Color: palette.queryBackground, Padding: woxwidget.Insets{Left: 16, Top: 18}, Child: woxwidget.Text{
			Value: "No application candidates are available on this platform.", Style: woxui.TextStyle{Size: 12}, Color: palette.actionHeader,
		}}
	} else {
		list = woxwidget.Gesture{ID: "form-table-app-scroll", OnScroll: func(delta woxui.Point) { a.scrollFormTableAppPicker(-delta.Y) }, Child: woxwidget.ScrollView{
			Width: width, Height: viewportHeight, ContentHeight: max(viewportHeight, float32(len(rows))*formTableAppPickerRowHeight), Offset: snapshot.scroll,
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
		}}
	}
	footer := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: []woxwidget.Widget{
		woxwidget.Container{Width: max(float32(0), width-112), Height: 42, Padding: woxwidget.Insets{Top: 13}, Child: woxwidget.Text{
			Value: "Select application · ↑↓ move · Enter choose · Esc cancel", Style: woxui.TextStyle{Size: 10}, Color: palette.actionHeader,
		}},
		a.buildFormTableButton("form-table-app-cancel", "Cancel", 104, true, false, a.closeFormTableAppPicker, palette),
	}}
	return woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{list, woxwidget.Container{Width: width, Height: footerHeight, Padding: woxwidget.Insets{Top: 8}, Child: footer}}}
}
