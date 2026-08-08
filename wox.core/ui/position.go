package ui

import (
	"context"
	"fmt"
	"wox/setting"
	"wox/util"
	"wox/util/screen"
)

// Position calculation for multi-monitor setups
//
// IMPORTANT: We use logical coordinates throughout the system for consistency.
// The screen package returns logical coordinates (DPI-adjusted), and we calculate
// window positions in logical space. The UI frontend will convert these back
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
//   3. Base heights shared with the Go UI (QueryBoxBaseHeight, ResultItemBaseHeight)
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
	x, y := getWindowMouseScreenLocation(util.NewTraceContext(), windowWidth, 0, true, true)
	return Position{
		Type: setting.PositionTypeMouseScreen,
		X:    x,
		Y:    y,
	}
}

func NewActiveScreenPosition(windowWidth int) Position {
	x, y := getWindowActiveScreenLocation(util.NewTraceContext(), windowWidth, 0, true, true)
	return Position{
		Type: setting.PositionTypeActiveScreen,
		X:    x,
		Y:    y,
	}
}

func NewMouseScreenPositionWithOptions(ctx context.Context, windowWidth int, maxResultCount int, showQueryBox bool, showToolbar bool) Position {
	x, y := getWindowMouseScreenLocation(ctx, windowWidth, maxResultCount, showQueryBox, showToolbar)
	return Position{
		Type: setting.PositionTypeMouseScreen,
		X:    x,
		Y:    y,
	}
}

func NewActiveScreenPositionWithOptions(ctx context.Context, windowWidth int, maxResultCount int, showQueryBox bool, showToolbar bool) Position {
	x, y := getWindowActiveScreenLocation(ctx, windowWidth, maxResultCount, showQueryBox, showToolbar)
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

func getWindowMouseScreenLocation(ctx context.Context, windowWidth int, maxResultCount int, showQueryBox bool, showToolbar bool) (int, int) {
	size := screen.GetMouseScreen()
	x, y := getCenterLocation(ctx, size, windowWidth, maxResultCount, showQueryBox, showToolbar)
	if util.IsLinux() {
		util.GetLogger().Info(ctx, fmt.Sprintf("linux-window-bounds go stage=mouse-screen screen=%d,%d %dx%d windowWidth=%d maxResultCount=%d showQueryBox=%t showToolbar=%t target=%d,%d screenDebug=%s", size.X, size.Y, size.Width, size.Height, windowWidth, maxResultCount, showQueryBox, showToolbar, x, y, screen.LastMouseScreenDebug()))
	}
	return x, y
}

func getWindowActiveScreenLocation(ctx context.Context, windowWidth int, maxResultCount int, showQueryBox bool, showToolbar bool) (int, int) {
	size := screen.GetActiveScreen()
	x, y := getCenterLocation(ctx, size, windowWidth, maxResultCount, showQueryBox, showToolbar)
	if util.IsLinux() {
		util.GetLogger().Info(ctx, fmt.Sprintf("linux-window-bounds go stage=active-screen screen=%d,%d %dx%d windowWidth=%d maxResultCount=%d showQueryBox=%t showToolbar=%t target=%d,%d screenDebug=%s", size.X, size.Y, size.Width, size.Height, windowWidth, maxResultCount, showQueryBox, showToolbar, x, y, screen.LastMouseScreenDebug()))
	}
	return x, y
}

func getCenterLocation(ctx context.Context, size screen.Size, windowWidth int, maxResultCount int, showQueryBox bool, showToolbar bool) (int, int) {
	maxWindowHeight := CalculateMaxWindowHeight(ctx, maxResultCount, showQueryBox, showToolbar)

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

func CalculateMaxWindowHeight(ctx context.Context, maxResultCount int, showQueryBox bool, showToolbar bool) int {
	theme := GetUIManager().GetCurrentTheme(ctx)
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)

	if maxResultCount <= 0 {
		maxResultCount = woxSetting.MaxResultCount.Get()
	}
	if maxResultCount <= 0 {
		maxResultCount = 10
	}

	queryBoxBaseHeight := DensityQueryBoxBaseHeight(ctx)
	resultItemBaseHeight := DensityResultItemBaseHeight(ctx)
	toolbarHeight := DensityToolbarHeight(ctx)

	queryBoxHeight := 0
	if showQueryBox {
		// Density scales only the shared launcher content heights. Theme
		// padding remains unchanged so normal density preserves the old formula
		// and custom themes keep their explicit spacing across density changes.
		queryBoxHeight = queryBoxBaseHeight + theme.AppPaddingTop + theme.AppPaddingBottom
	}

	resultItemHeight := resultItemBaseHeight + theme.ResultItemPaddingTop + theme.ResultItemPaddingBottom
	resultListViewHeight := resultItemHeight * maxResultCount
	resultContainerHeight := resultListViewHeight + theme.ResultContainerPaddingTop + theme.ResultContainerPaddingBottom

	extraToolbarHeight := 0
	if showToolbar {
		extraToolbarHeight = toolbarHeight
	}

	return queryBoxHeight + resultContainerHeight + extraToolbarHeight
}
