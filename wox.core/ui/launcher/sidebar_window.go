package launcher

import (
	woxui "wox/ui/runtime"
	"wox/util/screen"
)

// syncSidebarWindow applies a logical minimum and expands the restored window when needed.
// A maximized window stays on its display; the host separately constrains its restore frame.
func syncSidebarWindow(window *woxui.Window, minimum woxui.Size, maximized bool, clamp func(woxui.Rect) woxui.Rect, beforeResize func(woxui.Size)) error {
	if err := window.SetMinSize(minimum); err != nil {
		return err
	}
	if maximized {
		return nil
	}
	bounds, err := window.Bounds()
	if err != nil {
		return err
	}
	if bounds.Width >= minimum.Width {
		return nil
	}
	previous := bounds
	bounds.Width = minimum.Width
	bounds = clamp(bounds)
	bounds.Width = max(bounds.Width, minimum.Width)
	bounds = fitSidebarWindow(bounds, previous)
	if beforeResize != nil {
		beforeResize(woxui.Size{Width: bounds.Width, Height: bounds.Height})
	}
	return window.SetBounds(bounds)
}

func fitSidebarWindow(bounds, previous woxui.Rect) woxui.Rect {
	if displays, err := screen.ListDisplays(); err == nil && len(displays) > 0 {
		return fitSidebarWindowToDisplays(bounds, previous, displays)
	}
	return bounds
}

// fitSidebarWindowToDisplays retains the best-overlapping logical work area when expanding.
func fitSidebarWindowToDisplays(bounds, previous woxui.Rect, displays []screen.Display) woxui.Rect {
	if len(displays) == 0 {
		return bounds
	}
	best, bestArea := displays[0].WorkArea, -1
	for _, display := range displays {
		work := display.WorkArea
		left, top := max(int(previous.X), work.X), max(int(previous.Y), work.Y)
		right, bottom := min(int(previous.X+previous.Width), work.Right()), min(int(previous.Y+previous.Height), work.Bottom())
		area := max(0, right-left) * max(0, bottom-top)
		if area > bestArea || area == bestArea && display.Primary {
			best, bestArea = work, area
		}
	}
	bounds.X = max(float32(best.X), min(bounds.X, float32(best.Right())-bounds.Width))
	bounds.Y = max(float32(best.Y), min(bounds.Y, float32(best.Bottom())-bounds.Height))
	return bounds
}
