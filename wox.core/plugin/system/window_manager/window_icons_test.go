package window_manager

import (
	"strings"
	"testing"

	"wox/common/icons"
	woxsvg "wox/util/svg"
)

func TestWindowManagementIconsUseConsistentThemeAwareGlyphs(t *testing.T) {
	cases := []struct {
		op   windowOperation
		name string
	}{
		{operationLeftHalf, icons.LayoutLeftHalf}, {operationRightHalf, icons.LayoutRightHalf},
		{operationTopHalf, icons.LayoutTopHalf}, {operationBottomHalf, icons.LayoutBottomHalf},
		{operationTopLeftQuarter, icons.LayoutTopLeft}, {operationTopRightQuarter, icons.LayoutTopRight},
		{operationBottomLeftQuarter, icons.LayoutBottomLeft}, {operationBottomRightQuarter, icons.LayoutBottomRight},
		{operationFirstThird, icons.LayoutFirstThird}, {operationCenterThird, icons.LayoutCenterThird}, {operationLastThird, icons.LayoutLastThird},
		{operationFirstTwoThirds, icons.LayoutFirstTwoThirds}, {operationLastTwoThirds, icons.LayoutLastTwoThirds},
		{operationAlmostMaximize, icons.LayoutAlmostMaximize}, {operationCenter, icons.LayoutCenter},
		{operationReasonableSize, icons.LayoutReasonable}, {operationMaximizeHeight, icons.LayoutMaximizeHeight},
		{operationMaximizeWidth, icons.LayoutMaximizeWidth}, {operationMaximize, icons.WindowMaximize},
		{operationRestore, icons.WindowRestore}, {"restore-maximized", icons.WindowRestore},
		{operationMinimize, icons.WindowMinimize}, {"toggle-fullscreen", icons.WindowFullscreen},
		{"toggle-floating", icons.WindowFloating}, {"swap-left", icons.WindowSwapLeft}, {"swap-right", icons.WindowSwapRight},
		{"swap-up", icons.WindowSwapUp}, {"swap-down", icons.WindowSwapDown}, {"focus-window", icons.WindowFocus},
		{operationNextDisplay, icons.WindowNextDisplay}, {operationPreviousDisplay, icons.WindowPreviousDisplay},
		{"workspace-1", icons.WindowMoveWorkspace}, {"workspace-10", icons.WindowMoveWorkspace},
	}
	for _, tc := range cases {
		icon := windowManagerCommandIcon(tc.op)
		if icon.IsEmpty() || icon.ImageData != icons.Get(tc.name).ImageData {
			t.Fatalf("%s uses wrong icon", tc.op)
		}
		if !strings.Contains(icon.ImageData, `stroke="var(--wox-theme-icon-color)"`) {
			t.Fatalf("%s does not follow theme", tc.op)
		}
		for _, color := range []string{"#000000", "#ffffff"} {
			img, err := woxsvg.Render(strings.ReplaceAll(icon.ImageData, "var(--wox-theme-icon-color)", color), 24, 24)
			if err != nil {
				t.Fatalf("%s %s: %v", tc.op, color, err)
			}
			visible := false
			for y := 0; y < 24; y++ {
				for x := 0; x < 24; x++ {
					if img.RGBAAt(x, y).A > 0 {
						visible = true
					}
				}
			}
			if !visible {
				t.Fatalf("%s %s is blank", tc.op, color)
			}
		}
	}
	for _, pair := range [][2]windowOperation{{"swap-left", "swap-right"}, {"swap-up", "swap-down"}, {operationMaximize, "restore-maximized"}, {"toggle-fullscreen", "toggle-floating"}, {operationNextDisplay, operationPreviousDisplay}} {
		if windowManagerCommandIcon(pair[0]).ImageData == windowManagerCommandIcon(pair[1]).ImageData {
			t.Fatalf("%s and %s use identical glyphs", pair[0], pair[1])
		}
	}
}
