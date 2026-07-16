package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestLauncherWindowOriginPreservesDraggedPosition(t *testing.T) {
	params := showAppParams{Position: position{X: 400, Y: 300}}
	current := woxui.Rect{X: 92, Y: 74, Width: 760, Height: 420}

	x, y := launcherWindowOrigin(params, current, 620, false)
	if x != current.X || y != current.Y {
		t.Fatalf("preserved origin = %.0f,%.0f, want %.0f,%.0f", x, y, current.X, current.Y)
	}
}

func TestLauncherWindowOriginKeepsBottomQueryBoxAnchored(t *testing.T) {
	params := showAppParams{QueryBoxAtBottom: true}
	current := woxui.Rect{X: 92, Y: 200, Width: 760, Height: 420}

	x, y := launcherWindowOrigin(params, current, 620, false)
	if x != current.X || y != 0 {
		t.Fatalf("bottom-anchored origin = %.0f,%.0f, want %.0f,0", x, y, current.X)
	}
}

func TestLauncherWindowOriginUsesShowPositionWhenRequested(t *testing.T) {
	params := showAppParams{Position: position{X: 400, Y: 300}}
	current := woxui.Rect{X: 92, Y: 74, Width: 760, Height: 420}

	x, y := launcherWindowOrigin(params, current, 620, true)
	if x != 400 || y != 300 {
		t.Fatalf("show origin = %.0f,%.0f, want 400,300", x, y)
	}
}
