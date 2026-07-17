package view

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// dropdownIndicator centers Flutter's 10x6 arrow_drop_down geometry in the available icon slot.
func dropdownIndicator(width, height float32, color woxui.Color) woxwidget.Widget {
	return woxwidget.Painter{Width: width, Height: height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		triangleWidth := min(float32(10), bounds.Width)
		triangleHeight := min(float32(6), bounds.Height)
		left := bounds.X + (bounds.Width-triangleWidth)/2
		top := bounds.Y + (bounds.Height-triangleHeight)/2
		displayList.FillConvexPolygon([]woxui.Point{
			{X: left, Y: top},
			{X: left + triangleWidth, Y: top},
			{X: left + triangleWidth/2, Y: top + triangleHeight},
		}, color)
	}}
}
