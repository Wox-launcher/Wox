package ui

import (
	"wox/setting"
	"wox/util/screen"
)

// Position calculation for multi-monitor setups
//
// IMPORTANT: We use logical coordinates throughout the system for consistency.
// The screen package returns logical coordinates (DPI-adjusted), and we calculate
// window positions in logical space. The Flutter frontend will convert these back
// to physical coordinates using the correct monitor's DPI.
//
// Example with two monitors:
//   Monitor 1 (Primary): 5120x2880 @ 225% DPI → 2275x1280 logical, offset (0,0)
//   Monitor 2 (Right):   1920x1080 @ 100% DPI → 1920x1080 logical, offset (5120,1080) physical
//
// When calculating window position for Monitor 2:
//   - screen.GetMouseScreen() returns: {Width: 1920, Height: 1080, X: 5120, Y: 1080} in logical coords
//   - Window X: 5120 + (1920-800)/2 = 5680 logical
//   - Window Y: 1080 + 1080/7 = 1234 logical  ← MUST include Y offset!
//
// Common mistake (before fix):
//   - Window Y: 1080/7 = 154 logical  ← Missing Y offset, window appears off-screen!
//
// The Y offset is critical because:
//   1. Monitors can be arranged vertically or at different heights
//   2. Without the offset, windows always appear at the top of the virtual desktop
//   3. In the example above, Y=154 is outside Monitor 2's bounds [1080,2160]

type Position struct {
	Type setting.PositionType
	X    int
	Y    int
}

func NewMouseScreenPosition(windowWidth int) Position {
	x, y := getWindowMouseScreenLocation(windowWidth)
	return Position{
		Type: setting.PositionTypeMouseScreen,
		X:    x,
		Y:    y,
	}
}

func NewActiveScreenPosition(windowWidth int) Position {
	x, y := getWindowActiveScreenLocation(windowWidth)
	return Position{
		Type: setting.PositionTypeActiveScreen,
		X:    x,
		Y:    y,
	}
}

func NewLastLocationPosition(x, y int) Position {
	return Position{
		Type: setting.PositionTypeLastLocation,
		X:    x,
		Y:    y,
	}
}

func getWindowMouseScreenLocation(windowWidth int) (int, int) {
	size := screen.GetMouseScreen()
	// Center horizontally: monitor's left edge + (monitor width - window width) / 2
	x := size.X + (size.Width-windowWidth)/2
	// Position vertically: monitor's top edge + 1/7 of monitor height
	// CRITICAL: Must include size.Y (monitor's top offset) for multi-monitor setups
	// Example: Monitor at physical offset (5120,1080) with 100% DPI
	//   → logical offset (5120,1080), height 1080
	//   → y = 1080 + 1080/7 = 1234 (correct position on that monitor)
	//   → Without size.Y: y = 1080/7 = 154 (wrong! outside monitor bounds)
	y := size.Y + size.Height/7
	return x, y
}

func getWindowActiveScreenLocation(windowWidth int) (int, int) {
	size := screen.GetActiveScreen()
	x := size.X + (size.Width-windowWidth)/2
	y := size.Y + size.Height/7 // Same logic as above
	return x, y
}
