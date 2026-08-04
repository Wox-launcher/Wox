package component

import (
	"math"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// WoxLoadingIndicator renders Flutter's rotating query loading ring.
func WoxLoadingIndicator(size float32, color woxui.Color) woxwidget.Widget {
	if size <= 0 {
		size = 16
	}
	image := svgIconImage("control.loading", size, color)
	return woxwidget.LoopAnimation{Key: "wox-loading-indicator", Duration: 900 * time.Millisecond, Builder: func(progress float32) woxwidget.Widget {
		return woxwidget.Painter{Width: size, Height: size, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			displayList.DrawRotatedImage(image, bounds, progress*2*math.Pi)
		}}
	}}
}
