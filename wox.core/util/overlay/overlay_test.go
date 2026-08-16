package overlay

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestBoundsMovesUpWhenGrowthReachesWorkAreaBottom(t *testing.T) {
	workArea := woxui.Rect{Width: 1200, Height: 900}
	bounds := Bounds(WindowOptions{AbsolutePosition: true, Anchor: AnchorTopLeft, OffsetX: 300, OffsetY: 700}, woxui.Rect{}, workArea, woxui.Size{Width: 420, Height: 600})
	if bounds.X != 300 || bounds.Y != 300 {
		t.Fatalf("clamped origin = (%v, %v), want (300, 300)", bounds.X, bounds.Y)
	}
}

func TestBoundsPreservesPositionUntilClampingIsNeeded(t *testing.T) {
	workArea := woxui.Rect{X: -1000, Width: 1000, Height: 800}
	current := woxui.Rect{X: -900, Y: 120, Width: 300, Height: 100}
	bounds := Bounds(WindowOptions{PreservePosition: true}, current, workArea, woxui.Size{Width: 420, Height: 760})
	if bounds.X != -900 || bounds.Y != 40 {
		t.Fatalf("preserved and clamped origin = (%v, %v), want (-900, 40)", bounds.X, bounds.Y)
	}
}

func TestBoundsPlacesBelowAnchorTopAtWindowBottom(t *testing.T) {
	workArea := woxui.Rect{Width: 1920, Height: 1080}
	target := woxui.Rect{X: 200, Y: 100, Width: 1200, Height: 500}
	bounds := boundsForTarget(WindowOptions{Anchor: AnchorBelowCenter}, target, workArea, woxui.Size{Width: 500, Height: 48})
	if bounds.X != 550 || bounds.Y != 600 {
		t.Fatalf("below-center origin = (%v, %v), want (550, 600)", bounds.X, bounds.Y)
	}
}

func TestWorkAreaUsesTrackedPlatformCoordinates(t *testing.T) {
	explicit := woxui.Rect{X: -1920, Y: -180, Width: 1920, Height: 1080}
	if got := WorkArea(WindowOptions{WorkArea: &explicit}, woxui.Rect{}); got != explicit {
		t.Fatalf("explicit work area = %+v, want %+v", got, explicit)
	}
}

func TestRequestCloseFiresCallbackOnce(t *testing.T) {
	called := 0
	RegisterCloseCallback("test", func() { called++ })
	if callback := takeCloseCallback("test"); callback != nil {
		callback()
	}
	if callback := takeCloseCallback("test"); callback != nil {
		callback()
	}
	if called != 1 {
		t.Fatalf("close callback count = %d, want 1", called)
	}
}

func TestTooltipOverlayStaysNonactivating(t *testing.T) {
	options := overlayNativeWindowOptions(WindowOptions{Topmost: true}, woxui.Size{Width: 280, Height: 48})
	if options.Role != woxui.WindowRoleUtility {
		t.Fatalf("tooltip role = %d, want utility", options.Role)
	}
	if !options.Nonactivating {
		t.Fatal("tooltips must not steal focus")
	}
	if !options.TransientOverlay {
		t.Fatal("tooltips must use the shared overlay material")
	}
	if !options.Topmost {
		t.Fatal("tooltips must stay above the settings window")
	}
}

func TestPreviewOverlayFloatsAboveLauncher(t *testing.T) {
	options := overlayNativeWindowOptions(WindowOptions{Topmost: true, CloseOnEscape: true, TakeFocus: true, Resizable: true}, woxui.Size{Width: 400, Height: 280})
	if options.Role != woxui.WindowRoleUtility {
		t.Fatalf("overlay role = %d, want utility", options.Role)
	}
	if !options.Topmost {
		t.Fatal("preview overlays must stay above the launcher")
	}
	if options.Nonactivating {
		t.Fatal("escape-to-close preview overlays still take focus")
	}
	if !options.TransientOverlay {
		t.Fatal("focus-taking previews must keep the shared overlay material")
	}
	if !options.Resizable {
		t.Fatal("preview overlays should keep native resizing")
	}
}

func TestSyncOverlayAppearanceFollowsThemeWhenRequested(t *testing.T) {
	themed := WindowOptions{LightAppearance: true, FollowsThemeAppearance: true}
	if !syncOverlayAppearance(&themed, true) {
		t.Fatal("themed overlay should update native appearance")
	}
	if themed.LightAppearance {
		t.Fatal("dark theme should request a dark overlay appearance")
	}

	unthemed := WindowOptions{LightAppearance: true}
	if syncOverlayAppearance(&unthemed, true) {
		t.Fatal("unthemed overlay should keep its creation appearance")
	}
	if !unthemed.LightAppearance {
		t.Fatal("unthemed overlay LightAppearance should stay unchanged")
	}
}

func TestScaledBoundsPreservesCenterAndAspectRatio(t *testing.T) {
	current := woxui.Rect{X: 300, Y: 200, Width: 400, Height: 250}
	target := scaledBounds(current, woxui.Rect{Width: 1200, Height: 900}, 1.25, 1.6, woxui.Size{Width: 180, Height: 120})
	if target != (woxui.Rect{X: 250, Y: 168.75, Width: 500, Height: 312.5}) {
		t.Fatalf("scaled bounds = %+v", target)
	}
}
