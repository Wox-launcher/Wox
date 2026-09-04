package view

import (
	"fmt"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// formTableCellIcons composes prepared images with table-scoped hover targets.
func formTableCellIcons(props FormTableFieldProps, rowIndex, columnIndex int, cell FormTableCell) woxwidget.Widget {
	children := make([]woxwidget.Widget, 0, len(cell.Icons)+1)
	for index, item := range cell.Icons {
		var icon woxwidget.Widget = woxwidget.Container{Width: 18, Height: 18}
		if item.Source != nil {
			icon = woxwidget.Image{Source: item.Source, Width: 18, Height: 18}
		}
		if item.Tooltip != "" {
			tooltip := item.Tooltip
			icon = woxwidget.Gesture{ID: fmt.Sprintf("%s-row-%d-cell-%d-icon-%d", props.ID, rowIndex, columnIndex, index),
				OnHoverAt: func(inside bool, bounds woxui.Rect) {
					if props.OnTooltip != nil {
						props.OnTooltip(inside, tooltip, bounds)
					}
				}, Child: icon,
			}
		}
		children = append(children, icon)
	}
	if cell.IconOverflow > 0 {
		children = append(children, woxwidget.Text{Value: fmt.Sprintf("+%d", cell.IconOverflow), Style: woxui.TextStyle{Size: woxcomponent.SettingsSearchSubtitleFontSize}, Color: props.Theme.ResultSubtitle})
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 4, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children}
}
