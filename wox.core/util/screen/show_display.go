package screen

import "sort"

// ShowDisplayMatch is how a saved monitor was resolved against the current layout.
type ShowDisplayMatch int

const (
	// ShowDisplayMatchNone means no display could be enumerated.
	ShowDisplayMatchNone ShowDisplayMatch = iota
	// ShowDisplayMatchID means the saved platform id is still connected.
	ShowDisplayMatchID
	// ShowDisplayMatchGeometry means the id changed but a similar monitor stayed nearby.
	ShowDisplayMatchGeometry
	// ShowDisplayMatchFallback means the saved monitor is gone and a substitute was chosen.
	ShowDisplayMatchFallback
)

// SavedDisplay is the monitor fingerprint stored when the user picks a screen.
type SavedDisplay struct {
	ID         string
	WorkX      int
	WorkY      int
	WorkWidth  int
	WorkHeight int
	Primary    bool
}

// SortDisplays orders monitors left to right, then top to bottom, using the work area.
func SortDisplays(displays []Display) {
	sort.SliceStable(displays, func(i, j int) bool {
		left := showDisplaySortRect(displays[i])
		right := showDisplaySortRect(displays[j])
		if left.X == right.X {
			return left.Y < right.Y
		}
		return left.X < right.X
	})
}

// ResolveShowDisplay picks the monitor a saved choice should open on.
// A trusted id follows that output even after it moves. Geometry only repairs an id
// that changed while the panel stayed near its saved work area. Anything else falls
// back to the primary display.
func ResolveShowDisplay(displays []Display, saved SavedDisplay, trustID bool) (Display, ShowDisplayMatch) {
	if len(displays) == 0 {
		return Display{}, ShowDisplayMatchNone
	}
	if trustID && saved.ID != "" {
		for _, display := range displays {
			if display.ID == saved.ID {
				return display, ShowDisplayMatchID
			}
		}
	}
	if match, ok := matchShowDisplayGeometry(displays, saved); ok {
		return match, ShowDisplayMatchGeometry
	}
	for _, display := range displays {
		if display.Primary {
			return display, ShowDisplayMatchFallback
		}
	}
	return displays[0], ShowDisplayMatchFallback
}

// PointOnAnyDisplay reports whether a window origin lies inside a connected monitor.
// Full bounds are used so an origin in the system chrome still counts. An empty list
// cannot prove the point is gone.
func PointOnAnyDisplay(x, y int, displays []Display) bool {
	if len(displays) == 0 {
		return true
	}
	for _, display := range displays {
		bounds := display.Bounds
		if bounds.IsEmpty() {
			bounds = display.WorkArea
		}
		if bounds.IsEmpty() {
			continue
		}
		if x >= bounds.X && y >= bounds.Y && x < bounds.Right() && y < bounds.Bottom() {
			return true
		}
	}
	return false
}

func showDisplaySortRect(display Display) Rect {
	if !display.WorkArea.IsEmpty() {
		return display.WorkArea
	}
	return display.Bounds
}

func matchShowDisplayGeometry(displays []Display, saved SavedDisplay) (Display, bool) {
	if saved.WorkWidth <= 0 || saved.WorkHeight <= 0 {
		return Display{}, false
	}
	savedCenterX := saved.WorkX + saved.WorkWidth/2
	savedCenterY := saved.WorkY + saved.WorkHeight/2
	limit := max(saved.WorkWidth, saved.WorkHeight) / 2
	limitSq := int64(limit) * int64(limit)
	var near Display
	nearCount := 0
	for _, display := range displays {
		area := display.WorkArea
		if area.IsEmpty() {
			area = display.Bounds
		}
		if !showDisplaySizeClose(area.Width, saved.WorkWidth) || !showDisplaySizeClose(area.Height, saved.WorkHeight) {
			continue
		}
		dx := int64(area.X + area.Width/2 - savedCenterX)
		dy := int64(area.Y + area.Height/2 - savedCenterY)
		if dx*dx+dy*dy > limitSq {
			continue
		}
		near = display
		nearCount++
	}
	// Equal-sized monitors remain distinguishable when only one is near the saved location.
	if nearCount != 1 {
		return Display{}, false
	}
	return near, true
}

func showDisplaySizeClose(current, saved int) bool {
	if saved <= 0 || current <= 0 {
		return false
	}
	delta := current - saved
	if delta < 0 {
		delta = -delta
	}
	tolerance := max(64, (saved*12+50)/100)
	return delta <= tolerance
}
