package component

import (
	"fmt"
	"math"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// WoxProgressIndicator renders Flutter's compact toolbar progress ring.
func WoxProgressIndicator(size float32, progress int, indeterminate bool, color woxui.Color) woxwidget.Widget {
	if indeterminate {
		return WoxLoadingIndicator(size, color)
	}
	if size <= 0 {
		size = 14
	}
	progress = min(max(progress, 0), 100)
	source := toolbarProgressSVG(progress)
	return woxwidget.Image{Source: svgSourceImage(fmt.Sprintf("toolbar-progress-%d", progress), source, size, color), Width: size, Height: size}
}

func toolbarProgressSVG(progress int) string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="3.43"><circle cx="12" cy="12" r="9" opacity=".2"/><circle cx="12" cy="12" r="9" pathLength="100" stroke-dasharray="%d 100" transform="matrix(0 -1 -1 0 24 24)"/></svg>`, progress)
}

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
