package ui

import (
	"testing"

	"wox/setting"
)

func TestResolveQueryHotkeyWindowPositionSupportsNineGrid(t *testing.T) {
	tests := []struct {
		position setting.QueryHotkeyPosition
		x        int
		y        int
	}{
		{setting.QueryHotkeyPositionTopLeft, 10, 40},
		{setting.QueryHotkeyPositionTopCenter, 20, 40},
		{setting.QueryHotkeyPositionTopRight, 30, 40},
		{setting.QueryHotkeyPositionMiddleLeft, 10, 50},
		{setting.QueryHotkeyPositionCenter, 20, 50},
		{setting.QueryHotkeyPositionMiddleRight, 30, 50},
		{setting.QueryHotkeyPositionBottomLeft, 10, 60},
		{setting.QueryHotkeyPositionBottomCenter, 20, 60},
		{setting.QueryHotkeyPositionBottomRight, 30, 60},
	}
	for _, test := range tests {
		x, y, ok := resolveQueryHotkeyWindowPosition(test.position, 10, 20, 30, 40, 50, 60)
		if !ok || x != test.x || y != test.y {
			t.Errorf("position %q = (%d, %d, %v), want (%d, %d, true)", test.position, x, y, ok, test.x, test.y)
		}
	}
	if _, _, ok := resolveQueryHotkeyWindowPosition(setting.QueryHotkeyPositionSystemDefault, 10, 20, 30, 40, 50, 60); ok {
		t.Fatal("system default should defer to the global position")
	}
}

func TestNormalizeQueryHotkeyPositionPreservesMiddlePositions(t *testing.T) {
	for _, position := range []setting.QueryHotkeyPosition{
		setting.QueryHotkeyPositionMiddleLeft,
		setting.QueryHotkeyPositionMiddleRight,
	} {
		if got := normalizeQueryHotkeyPosition(string(position)); got != position {
			t.Errorf("normalize %q = %q, want unchanged", position, got)
		}
	}
}
