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
func specificScreenPosition(ctx context.Context, windowWidth int, maxResultCount int, showQueryBox bool, showToolbar bool, saved setting.ShowDisplayTarget, trustID bool) (int, int, string) {
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
func centerOnShowDisplay(ctx context.Context, display screen.Display, windowWidth int, maxResultCount int, showQueryBox bool, showToolbar bool) (int, int, string) {
	area := display.WorkArea
	if area.IsEmpty() {
		area = display.Bounds
	}
	if area.IsEmpty() {
		return getWindowMouseScreenLocation(ctx, windowWidth, maxResultCount, showQueryBox, showToolbar)
	}
	x, y := getCenterLocation(ctx, screen.Size{X: area.X, Y: area.Y, Width: area.Width, Height: area.Height}, windowWidth, maxResultCount, showQueryBox, showToolbar)
	return x, y, display.ID
}

// resolveLastLocation restores a monitor-relative origin without rewriting legacy settings.
func resolveLastLocation(last setting.SavedWindowPosition, width, height int) (Position, bool) {
	if !last.Valid {
		return Position{}, false
	}
	displays, err := screen.ListDisplays()
	if err != nil || len(displays) == 0 {
		return Position{Type: setting.PositionTypeLastLocation, X: last.X, Y: last.Y, DisplayID: last.DisplayID}, true
	}
	return resolveLastLocationOnDisplays(last, displays, width, height)
}

// resolveLastLocationOnDisplays rejects disconnected or completely off-screen windows.
func resolveLastLocationOnDisplays(last setting.SavedWindowPosition, displays []screen.Display, width, height int) (Position, bool) {
	if !last.Valid {
		return Position{}, false
	}
	if last.Width > 0 {
		width = last.Width
	}
	if last.Height > 0 {
		height = last.Height
	}
	if last.DisplayID != "" {
		for _, display := range displays {
			if display.ID != last.DisplayID {
				continue
			}
			area := display.WorkArea
			if area.IsEmpty() {
				area = display.Bounds
			}
			x, y := last.X+area.X-last.WorkX, last.Y+area.Y-last.WorkY
			if lastLocationIntersectsDisplay(x, y, width, height, display) {
				return Position{Type: setting.PositionTypeLastLocation, X: x, Y: y, DisplayID: display.ID}, true
			}
			return Position{}, false
		}
		return Position{}, false
	}
	for _, display := range displays {
		if lastLocationIntersectsDisplay(last.X, last.Y, width, height, display) {
			return NewLastLocationPosition(last.X, last.Y), true
		}
	}
	return Position{}, false
}

// lastLocationIntersectsDisplay accepts a partly visible window even when its origin is off-screen.
func lastLocationIntersectsDisplay(x, y, width, height int, display screen.Display) bool {
	area := display.Bounds
	if area.IsEmpty() {
		area = display.WorkArea
	}
	return !area.IsEmpty() && width > 0 && height > 0 && x < area.Right() && x+width > area.X && y < area.Bottom() && y+height > area.Y
}
