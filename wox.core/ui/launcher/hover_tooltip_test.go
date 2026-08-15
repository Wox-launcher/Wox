package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestNativeHoverTooltipIgnoresOwnerPointerLeave(t *testing.T) {
	anchor := woxui.Rect{X: 10, Y: 20, Width: 80, Height: 26}
	if nativeHoverTooltipExplicitDismiss(false, "Copied at", anchor) {
		t.Fatal("pointer-leave from the owner window must not dismiss the native tooltip")
	}
	if nativeHoverTooltipExplicitDismiss(true, "Copied at", anchor) {
		t.Fatal("hover enter must show rather than dismiss")
	}
}

func TestNativeHoverTooltipDismissesOnExplicitClose(t *testing.T) {
	if !nativeHoverTooltipExplicitDismiss(false, "", woxui.Rect{}) {
		t.Fatal("empty close from window hide must dismiss the native tooltip")
	}
	if !nativeHoverTooltipExplicitDismiss(false, "Copied at", woxui.Rect{}) {
		t.Fatal("leave without an anchor must dismiss the native tooltip")
	}
}
