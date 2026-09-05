package overlay

import (
	"runtime"
	"testing"

	"wox/common"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/screen"
	"wox/util/window"
)

// TestStickyWindowIdentityAndDismissal covers PID fallback and retaining the last
// position when the dialog disappears on a secondary display.
func TestStickyWindowIdentityAndDismissal(t *testing.T) {
	options := WindowOptions{StickyWindowPid: 2147483647, StickyWindowId: "123", Anchor: AnchorBelowCenter}
	target := window.ManagedWindow{Id: "456", Pid: options.StickyWindowPid, Bounds: window.WindowRect{X: -1800, Y: -150, Width: 900, Height: 600}}
	displays := []screen.Display{{PixelBounds: screen.Rect{X: -1920, Y: -300, Width: 1920, Height: 1080}, Scale: 1.5}}
	if _, ok := stickyWindowBounds(options, target, displays); ok {
		t.Fatal("overlay followed a different window in the same process")
	}
	target.Id = options.StickyWindowId
	want := woxui.Rect{X: -1800, Y: -150, Width: 900, Height: 600}
	if runtime.GOOS == "windows" {
		want = woxui.Rect{X: -1200, Y: -100, Width: 600, Height: 400}
	}
	if got, ok := stickyWindowBounds(options, target, displays); !ok || got != want {
		t.Fatalf("tracked window bounds = %+v, %v, want %+v", got, ok, want)
	}
	options.StickyWindowId = ""
	if _, ok := stickyWindowBounds(options, target, displays); !ok {
		t.Fatal("PID-only tracking should still accept the resolved window")
	}
	options.StickyWindowId = "0"
	current := woxui.Rect{X: -1100, Y: 350, Width: 400, Height: 48}
	primaryWorkArea := woxui.Rect{Width: 1920, Height: 1080}
	got := Bounds(options, current, primaryWorkArea, woxui.Size{Width: current.Width, Height: current.Height})
	if got != current {
		t.Fatalf("dismissed dialog moved overlay to %+v, want %+v", got, current)
	}
}

// A missing/hidden dialog must not reach window layout, Show, or offset publication.
func TestInitialStickyLayoutWaitsForTarget(t *testing.T) {
	instance := &runtimeOverlay{
		options:       WindowOptions{StickyWindowPid: 2147483647, StickyWindowId: "0"},
		stickyPublish: func() { t.Fatal("published an offset before the target was ready") },
	}
	instance.applyLayout(false)
	if instance.shown {
		t.Fatal("overlay became visible before its target")
	}
}

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

func TestCloseUnknownOverlayDoesNotRequireUIRuntime(t *testing.T) {
	Close("wox_tooltip_never_opened")
	Close("wox_tooltip_never_opened")
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

func TestPanelFillIsOpaqueOnLinuxOnly(t *testing.T) {
	if color := PanelFill("windows", false); color.A != 0 {
		t.Fatalf("windows panel fill = %#v, want empty so acrylic shows through", color)
	}
	if color := PanelFill("darwin", false); color.A != 0 {
		t.Fatalf("darwin panel fill = %#v, want empty so vibrancy shows through", color)
	}
	dark := PanelFill("linux", false)
	if dark.A != 255 {
		t.Fatalf("linux dark panel fill = %#v, want opaque", dark)
	}
	light := PanelFill("linux", true)
	if light.A != 255 {
		t.Fatalf("linux light panel fill = %#v, want opaque", light)
	}
	if light == dark {
		t.Fatal("linux light and dark overlay fills must differ")
	}
}

func TestSurfaceFillKeepsThemeRGBButDropsLinuxAlpha(t *testing.T) {
	translucent := woxui.Color{R: 22, G: 22, B: 26, A: 132}
	if got := SurfaceFill("windows", translucent, false); got != translucent {
		t.Fatalf("windows surface fill = %#v, want theme alpha for acrylic", got)
	}
	if got := SurfaceFill("darwin", translucent, false); got != translucent {
		t.Fatalf("darwin surface fill = %#v, want theme alpha for vibrancy", got)
	}
	opaque := surfaceFill("linux", translucent, false, false)
	if opaque != (woxui.Color{R: 22, G: 22, B: 26, A: 255}) {
		t.Fatalf("linux surface fill = %#v, want opaque theme RGB", opaque)
	}
	if got := surfaceFill("linux", woxui.Color{}, true, false); got != PanelFill("linux", true) {
		t.Fatalf("linux empty surface fill = %#v, want light PanelFill", got)
	}
}

func TestSurfaceFillKeepsLinuxAlphaWhenNativeMaterialIsAvailable(t *testing.T) {
	translucent := woxui.Color{R: 22, G: 22, B: 26, A: 132}
	if got := surfaceFill("linux", translucent, false, true); got != translucent {
		t.Fatalf("linux surface fill with compositor blur = %#v, want theme alpha", got)
	}
	if got := surfaceFill("linux", woxui.Color{}, true, true); got != (woxui.Color{}) {
		t.Fatalf("linux empty surface fill with compositor blur = %#v, want empty so blur shows through", got)
	}
}

func TestHUDSurfaceUsesThemeBackground(t *testing.T) {
	child := woxwidget.Container{Width: 10, Height: 10}
	panel := HUDSurface(120, 48, 12, false, child)
	if panel.Width != 120 || panel.Height != 48 || panel.Radius != woxui.NativeWindowCornerRadius(12) {
		t.Fatalf("hud surface geometry = %+v, want 120x48 radius %v", panel, woxui.NativeWindowCornerRadius(12))
	}
	if panel.Color != CurrentThemeChrome().Background {
		t.Fatalf("hud surface fill = %#v, want theme AppBackgroundColor %#v", panel.Color, CurrentThemeChrome().Background)
	}
	if panel.Child != child {
		t.Fatal("hud surface dropped its child")
	}
}

func TestHUDSurfaceIsOpaqueOnLinuxWithTranslucentTheme(t *testing.T) {
	SetThemeProvider(func() common.Theme {
		return common.Theme{AppBackgroundColor: "rgba(22, 22, 26, 0.52)"}
	})
	defer SetThemeProvider(nil)
	panel := HUDSurface(120, 48, 12, false, woxwidget.Container{})
	if panel.Color != CurrentThemeChrome().Background {
		t.Fatalf("hud surface fill = %#v, want chrome %#v", panel.Color, CurrentThemeChrome().Background)
	}
	if runtime.GOOS == "linux" && !woxui.HasNativeWindowMaterial() && panel.Color.A != 255 {
		t.Fatalf("linux hud surface fill = %#v, want opaque when compositor blur is unavailable", panel.Color)
	}
	if runtime.GOOS != "linux" && panel.Color.A == 255 {
		t.Fatalf("non-linux hud surface fill = %#v, want theme alpha so acrylic shows through", panel.Color)
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
