//go:build linux

package woxui

import "testing"

func TestLinuxCustomChromeCornerRadiusClipsAuthoredShape(t *testing.T) {
	if got := testLinuxCustomChromeCornerRadius(false, 28); got != 0 {
		t.Fatalf("default linux chrome radius = %v, want 0 because compositor blur is rectangular", got)
	}
	if got := testLinuxCustomChromeCornerRadius(true, -1); got != DefaultWindowCornerRadius {
		t.Fatalf("custom chrome without an authored radius = %v, want the default rounded clip %v", got, DefaultWindowCornerRadius)
	}
	if got := testLinuxCustomChromeCornerRadius(true, 28); got != 28 {
		t.Fatalf("jade custom chrome radius = %v, want the authored 28px clip", got)
	}
	if got := testLinuxCustomChromeCornerRadius(true, 0); got != 0 {
		t.Fatalf("explicit square custom chrome radius = %v, want 0", got)
	}
}
