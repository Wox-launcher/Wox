//go:build windows

package woxui

import (
	"testing"

	"github.com/lxn/win"
)

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

func TestConstrainWindowsAspectRatio(t *testing.T) {
	bounds := win.RECT{Left: 0, Top: 0, Right: 300, Bottom: 100}
	constrainWindowsAspectRatio(2, &bounds, 2)
	if bounds.Bottom != 150 {
		t.Fatalf("constrained bounds = %+v, want height 150", bounds)
	}
}

func TestWindowsPhysicalMinSizeUsesWindowScale(t *testing.T) {
	width, height := windowsPhysicalMinSize(Size{Width: 460, Height: 240}, 1.5)
	if width != 690 || height != 360 {
		t.Fatalf("physical min size = %dx%d, want 690x360", width, height)
	}
}

func TestApplyWindowsMinTrackSizeRaisesSystemFloor(t *testing.T) {
	info := win.MINMAXINFO{PtMinTrackSize: win.POINT{X: 112, Y: 27}}
	applyWindowsMinTrackSize(&info, 690, 360)
	if info.PtMinTrackSize.X != 690 || info.PtMinTrackSize.Y != 360 {
		t.Fatalf("min track size = %+v, want 690x360", info.PtMinTrackSize)
	}
}

func TestConstrainWindowsMinSizeKeepsDraggedEdge(t *testing.T) {
	bounds := win.RECT{Left: 100, Top: 40, Right: 220, Bottom: 200}
	constrainWindowsMinSize(1, &bounds, 460, 240)
	if bounds.Left != bounds.Right-460 || bounds.Right != 220 {
		t.Fatalf("left-edge min width = %+v, want width 460 with right edge 220", bounds)
	}

	bounds = win.RECT{Left: 10, Top: 80, Right: 200, Bottom: 200}
	constrainWindowsMinSize(3, &bounds, 460, 240)
	if bounds.Top != bounds.Bottom-240 || bounds.Bottom != 200 {
		t.Fatalf("top-edge min height = %+v, want height 240 with bottom edge 200", bounds)
	}
}

func TestWindowsResizeHitTest(t *testing.T) {
	bounds := win.RECT{Right: 300, Bottom: 200}
	tests := []struct {
		position win.POINT
		want     uintptr
	}{
		{position: win.POINT{X: 5, Y: 5}, want: win.HTTOPLEFT},
		{position: win.POINT{X: 295, Y: 5}, want: win.HTTOPRIGHT},
		{position: win.POINT{X: 5, Y: 195}, want: win.HTBOTTOMLEFT},
		{position: win.POINT{X: 295, Y: 195}, want: win.HTBOTTOMRIGHT},
		{position: win.POINT{X: 150, Y: 5}, want: win.HTTOP},
		{position: win.POINT{X: 150, Y: 100}, want: win.HTCLIENT},
	}
	for _, test := range tests {
		if got := windowsResizeHitTest(test.position, bounds, 10); got != test.want {
			t.Fatalf("resize hit at %+v = %d, want %d", test.position, got, test.want)
		}
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
	if windowsWindowUsesSystemBackdrop(WindowOptions{Role: WindowRoleScreenshot}) {
		t.Fatal("screenshot window should not expose a system backdrop before its first frame")
	}
	for _, role := range []WindowRole{WindowRoleUtility, WindowRoleApplication} {
		if !windowsWindowUsesSystemBackdrop(WindowOptions{Role: role}) {
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
