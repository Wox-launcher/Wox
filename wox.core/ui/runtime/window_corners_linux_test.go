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

func TestLinuxUtilityWindowsKeepPerPixelAlphaForCustomChrome(t *testing.T) {
	if !testLinuxWindowUsesPerPixelAlpha(false, false, false, false) {
		t.Fatal("launcher windows must request RGBA so Jade can punch rounded corners without compositor blur")
	}
	if testLinuxWindowUsesPerPixelAlpha(true, false, false, false) {
		t.Fatal("application windows stay opaque when compositor blur is unavailable")
	}
	if !testLinuxWindowUsesPerPixelAlpha(true, false, false, true) {
		t.Fatal("application windows request RGBA when compositor blur is available")
	}
	if !testLinuxWindowUsesPerPixelAlpha(true, true, false, false) {
		t.Fatal("nonactivating windows request RGBA so overlays can be transparent")
	}
	if !testLinuxWindowUsesPerPixelAlpha(true, false, true, false) {
		t.Fatal("screenshot windows request RGBA so the desktop shows through")
	}
}
