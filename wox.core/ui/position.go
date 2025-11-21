package ui

import (
	"wox/setting"
	"wox/util"
	"wox/util/screen"
)

// Position calculation for multi-monitor setups
//
// IMPORTANT: We use logical coordinates throughout the system for consistency.
// The screen package returns logical coordinates (DPI-adjusted), and we calculate
// window positions in logical space. The Flutter frontend will convert these back
// to physical coordinates using the correct monitor's DPI.
//
// Window positioning strategy:
//   - X position: Centered horizontally on the screen
//   - Y position: Centered vertically based on the maximum window height
//
// Maximum window height calculation:
//   The window height is calculated based on:
//   1. User's configured maximum result count (MaxResultCount)
//   2. Theme padding values (AppPadding, ResultContainerPadding, ResultItemPadding)
//   3. Base heights from Flutter UI constants (QueryBoxBaseHeight, ResultItemBaseHeight)
//
//   Formula:
//   - QueryBoxHeight = QueryBoxBaseHeight + AppPaddingTop + AppPaddingBottom
//   - ResultItemHeight = ResultItemBaseHeight + ResultItemPaddingTop + ResultItemPaddingBottom
//   - ResultListViewHeight = ResultItemHeight × MaxResultCount
//   - ResultContainerHeight = ResultListViewHeight + ResultContainerPaddingTop + ResultContainerPaddingBottom
//   - MaxWindowHeight = QueryBoxHeight + ResultContainerHeight + ToolbarHeight
//
// Example with two monitors:
//   Monitor 1 (Primary): 5120x2880 @ 225% DPI → 2275x1280 logical, offset (0,0)
//   Monitor 2 (Right):   1920x1080 @ 100% DPI → 1920x1080 logical, offset (5120,1080) physical
//
// When calculating window position for Monitor 2:
//   - screen.GetMouseScreen() returns: {Width: 1920, Height: 1080, X: 5120, Y: 1080} in logical coords
//   - Window X: 5120 + (1920-800)/2 = 5680 logical (centered horizontally)
//   - Window Y: 5120 + (1080-maxWindowHeight)/2 (centered vertically based on max height)
//
// The Y offset is critical because:
//   1. Monitors can be arranged vertically or at different heights
//   2. Without the offset, windows always appear at the top of the virtual desktop
//   3. The window should be centered based on its maximum possible height for consistency

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
	return getCenterLocation(size, windowWidth)
}

func getWindowActiveScreenLocation(windowWidth int) (int, int) {
	size := screen.GetActiveScreen()
	return getCenterLocation(size, windowWidth)
}

func getCenterLocation(size screen.Size, windowWidth int) (int, int) {
	ctx := util.NewTraceContext()

	// Get current theme and settings
	theme := GetUIManager().GetCurrentTheme(ctx)
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)

	// Calculate maximum window height based on user configuration
	maxResultCount := woxSetting.MaxResultCount.Get()
	if maxResultCount == 0 {
		maxResultCount = 10 // Default value
	}

	// Constants from Flutter UI (consts.dart)
	const (
		queryBoxBaseHeight   = 55
		resultItemBaseHeight = 50
		toolbarHeight        = 40
	)

	// Calculate query box height (includes app padding top and bottom)
	queryBoxHeight := queryBoxBaseHeight + theme.AppPaddingTop + theme.AppPaddingBottom

	// Calculate result item height
	resultItemHeight := resultItemBaseHeight + theme.ResultItemPaddingTop + theme.ResultItemPaddingBottom

	// Calculate result list view height
	resultListViewHeight := resultItemHeight * maxResultCount

	// Calculate result container height (includes padding)
	resultContainerHeight := resultListViewHeight + theme.ResultContainerPaddingTop + theme.ResultContainerPaddingBottom

	// Calculate total maximum window height (including toolbar)
	// Note: Toolbar is shown when there are results, so we include it in max height calculation
	maxWindowHeight := queryBoxHeight + resultContainerHeight + toolbarHeight

	// Calculate X position (centered horizontally)
	x := size.X + (size.Width-windowWidth)/2

	// Calculate Y position: position the window so that when it reaches max height,
	// it will be vertically centered. This means the window's top position is fixed,
	// and it expands downward as results are added.
	//
	// Strategy: Place the query box at a position where the fully expanded window
	// would be centered. This creates a consistent anchor point.
	y := size.Y + (size.Height-maxWindowHeight)/2

	return x, y
}
