package ui

import (
	"context"

	"wox/setting"
	"wox/util/screen"
)

func savedShowDisplay(target setting.ShowDisplayTarget) screen.SavedDisplay {
	return screen.SavedDisplay{
		ID: target.ID, WorkX: target.WorkX, WorkY: target.WorkY,
		WorkWidth: target.WorkWidth, WorkHeight: target.WorkHeight, Primary: target.Primary,
	}
}

// specificScreenPosition centers the launcher on the resolved monitor's current work area.
func specificScreenPosition(ctx context.Context, windowWidth int, maxResultCount int, showQueryBox bool, showToolbar bool, saved setting.ShowDisplayTarget, trustID bool) (int, int) {
	displays, err := screen.ListDisplays()
	if err != nil || len(displays) == 0 {
		return getWindowMouseScreenLocation(ctx, windowWidth, maxResultCount, showQueryBox, showToolbar)
	}
	display, match := screen.ResolveShowDisplay(displays, savedShowDisplay(saved), trustID)
	if match == screen.ShowDisplayMatchNone {
		return getWindowMouseScreenLocation(ctx, windowWidth, maxResultCount, showQueryBox, showToolbar)
	}
	return centerOnShowDisplay(ctx, display, windowWidth, maxResultCount, showQueryBox, showToolbar)
}

// centerOnShowDisplay places the window in one monitor's current logical work area.
func centerOnShowDisplay(ctx context.Context, display screen.Display, windowWidth int, maxResultCount int, showQueryBox bool, showToolbar bool) (int, int) {
	area := display.WorkArea
	if area.IsEmpty() {
		area = display.Bounds
	}
	if area.IsEmpty() {
		return getWindowMouseScreenLocation(ctx, windowWidth, maxResultCount, showQueryBox, showToolbar)
	}
	return getCenterLocation(ctx, screen.Size{X: area.X, Y: area.Y, Width: area.Width, Height: area.Height}, windowWidth, maxResultCount, showQueryBox, showToolbar)
}

// lastLocationVisible reports whether a saved window origin still lies on a connected monitor.
func lastLocationVisible(x, y int) bool {
	displays, err := screen.ListDisplays()
	if err != nil || len(displays) == 0 {
		return true
	}
	return screen.PointOnAnyDisplay(x, y, displays)
}
