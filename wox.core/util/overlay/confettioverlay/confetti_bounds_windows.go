package confettioverlay

import (
	woxui "wox/ui/runtime"
	"wox/util/screen"
)

func confettiDisplaySize(display screen.Display) woxui.Size {
	return woxui.Size{Width: float32(display.PixelWorkArea.Width), Height: float32(display.PixelWorkArea.Height)}
}

func setConfettiDisplayBounds(window *woxui.Window, display screen.Display) error {
	area := display.PixelWorkArea
	return window.SetPhysicalBounds(woxui.Rect{X: float32(area.X), Y: float32(area.Y), Width: float32(area.Width), Height: float32(area.Height)})
}
