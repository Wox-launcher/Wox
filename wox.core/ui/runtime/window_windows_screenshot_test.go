//go:build windows

package woxui

import "testing"

func TestWindowsScreenshotWindowUsesPhysicalPixelScale(t *testing.T) {
	for _, dpi := range []uint32{96, 120, 144, 192} {
		if scale := windowsWindowScale(WindowRoleScreenshot, dpi); scale != 1 {
			t.Fatalf("screenshot scale at %d DPI = %v, want 1", dpi, scale)
		}
	}

	if scale := windowsWindowScale(WindowRoleApplication, 144); scale != 1.5 {
		t.Fatalf("application scale at 144 DPI = %v, want 1.5", scale)
	}
}

func TestWindowsScreenshotSelectionConvertsBackToLogicalBounds(t *testing.T) {
	physical := Rect{X: -1920, Y: 120, Width: 900, Height: 600}
	logical := windowsLogicalRectAtScale(physical, 1.5)
	want := Rect{X: -1280, Y: 80, Width: 600, Height: 400}
	if logical != want {
		t.Fatalf("logical bounds = %+v, want %+v", logical, want)
	}
}

func TestWindowsScreenshotRendererSkipsEmbeddedSurfaceOverlay(t *testing.T) {
	if windowsRendererNeedsEmbeddedSurfaceOverlay(WindowRoleScreenshot) {
		t.Fatal("screenshot renderer should not allocate an embedded-surface overlay")
	}
	for _, role := range []WindowRole{WindowRoleUtility, WindowRoleApplication} {
		if !windowsRendererNeedsEmbeddedSurfaceOverlay(role) {
			t.Fatalf("window role %d still requires embedded-surface overlay support", role)
		}
	}
}

func TestWindowsScreenshotWindowSkipsSystemBackdrop(t *testing.T) {
	if windowsWindowUsesSystemBackdrop(WindowRoleScreenshot) {
		t.Fatal("screenshot window should not expose a system backdrop before its first frame")
	}
	for _, role := range []WindowRole{WindowRoleUtility, WindowRoleApplication} {
		if !windowsWindowUsesSystemBackdrop(role) {
			t.Fatalf("window role %d still requires its system backdrop", role)
		}
	}
}

func TestWindowsConvertDIBToRGBA(t *testing.T) {
	pixels := []byte{30, 20, 10, 0, 90, 80, 70, 12}
	windowsConvertDIBToRGBA(pixels)
	want := []byte{10, 20, 30, 255, 70, 80, 90, 255}
	for index := range want {
		if pixels[index] != want[index] {
			t.Fatalf("converted pixels = %v, want %v", pixels, want)
		}
	}
}
