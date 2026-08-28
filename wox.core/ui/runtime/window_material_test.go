package woxui

import (
	"runtime"
	"testing"
)

func TestWindowUsesDefaultMaterialExceptScreenshot(t *testing.T) {
	if !windowUsesDefaultMaterial(WindowRoleUtility) || !windowUsesDefaultMaterial(WindowRoleApplication) {
		t.Fatal("utility and application windows must inherit the process material")
	}
	if windowUsesDefaultMaterial(WindowRoleScreenshot) {
		t.Fatal("screenshot windows must keep showing the desktop")
	}
}

func TestHasNativeWindowMaterialIsPlatformDefault(t *testing.T) {
	if runtime.GOOS == "linux" && HasNativeWindowMaterial() {
		t.Fatal("linux unit tests must not assume compositor blur before GTK starts")
	}
	if runtime.GOOS != "linux" && !HasNativeWindowMaterial() {
		t.Fatal("windows and macOS always have native window material")
	}
}

func TestNativeWindowCornerRadiusSquaresLinuxBlurWindows(t *testing.T) {
	if runtime.GOOS == "linux" && !HasNativeWindowMaterial() {
		if got := NativeWindowCornerRadius(14); got != 14 {
			t.Fatalf("linux without compositor blur radius = %v, want the painted outline", got)
		}
		return
	}
	if runtime.GOOS == "linux" {
		if got := NativeWindowCornerRadius(14); got != 0 {
			t.Fatalf("linux compositor blur radius = %v, want 0 because the protocol is rectangular", got)
		}
		return
	}
	if got := NativeWindowCornerRadius(14); got != 14 {
		t.Fatalf("non-linux window radius = %v, want the requested outline", got)
	}
}

func TestSetDefaultAppearanceIsProcessWide(t *testing.T) {
	previous := DefaultAppearanceIsDark()
	t.Cleanup(func() { SetDefaultAppearance(previous) })

	SetDefaultAppearance(false)
	if DefaultAppearanceIsDark() {
		t.Fatal("default appearance should be light")
	}
	SetDefaultAppearance(true)
	if !DefaultAppearanceIsDark() {
		t.Fatal("default appearance should be dark")
	}
}
