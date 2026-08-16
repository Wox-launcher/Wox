package macospermission

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/permission"
)

func TestOpenRejectsInvalidPermissionType(t *testing.T) {
	if err := Open(Options{PermissionType: "camera"}); err == nil {
		t.Fatal("invalid permission type must be rejected before System Settings opens")
	}
}

func TestPanelLayoutPrefersRightEdge(t *testing.T) {
	layout := PanelLayout(Rect{X: 100, Y: 100, Width: 800, Height: 600}, 330, 184, Rect{Width: 1440, Height: 900})
	if layout != (Layout{X: 900, Y: 100, Placement: PlacementRight}) {
		t.Fatalf("right layout = %+v", layout)
	}
}

func TestPanelLayoutUsesBottomWhenTrailingSpaceIsTight(t *testing.T) {
	layout := PanelLayout(Rect{X: 0, Y: 100, Width: 1400, Height: 600}, 330, 184, Rect{Width: 1440, Height: 900})
	if layout != (Layout{X: 1070, Y: 700, Placement: PlacementBottom}) {
		t.Fatalf("bottom layout = %+v", layout)
	}
}

func TestPanelLayoutDoesNotCoverPermissionListWhenNeitherEdgeFits(t *testing.T) {
	layout := PanelLayout(Rect{X: 0, Y: 30, Width: 1200, Height: 850}, 330, 184, Rect{Width: 1440, Height: 900})
	if layout.Placement != PlacementRight || layout.X != 1200 {
		t.Fatalf("fallback layout = %+v, want the roomier outside edge", layout)
	}
}

func TestPanelLayoutPreservesNegativeDesktopOrigin(t *testing.T) {
	layout := PanelLayout(Rect{X: -1800, Y: 100, Width: 800, Height: 600}, 330, 184, Rect{X: -1920, Width: 1920, Height: 1080})
	if layout != (Layout{X: -1000, Y: 100, Placement: PlacementRight}) {
		t.Fatalf("negative-origin layout = %+v", layout)
	}
}

func TestSettingsAnchorRemainsOwnedByPermissionPackage(t *testing.T) {
	if permission.MacOSPermissionSettingsAnchor(permission.MacOSPermissionFullDiskAccess) != "Privacy_AllFiles" {
		t.Fatal("full disk access anchor changed")
	}
}

func TestPermissionContentUsesFullPanelCenterline(t *testing.T) {
	instance := &session{opts: Options{Title: "Accessibility"}}
	root := instance.build(nil, woxui.FrameInfo{Size: woxui.Size{Width: panelWidth, Height: panelHeightManual}}, "Manual setup")
	stack := root.(woxwidget.Stack)
	contentAlign, ok := stack.Children[0].Child.(woxwidget.Align)
	if !ok {
		t.Fatalf("content root = %T, want full-panel Align", stack.Children[0].Child)
	}
	if contentAlign.Width != panelWidth || contentAlign.Horizontal != 0.5 {
		t.Fatalf("content alignment = width %v horizontal %v, want panel width %v centered", contentAlign.Width, contentAlign.Horizontal, panelWidth)
	}
}

func TestRuntimeDisposalNotifiesOwnerOnce(t *testing.T) {
	closed := 0
	instance := &session{stop: make(chan struct{}), opts: Options{OnClosed: func() { closed++ }}}
	activeSession.Lock()
	activeSession.current = instance
	activeSession.Unlock()

	instance.closedByRuntime()
	instance.closedByRuntime()
	if closed != 1 {
		t.Fatalf("runtime disposal callback count = %d, want 1", closed)
	}
	activeSession.Lock()
	current := activeSession.current
	activeSession.Unlock()
	if current != nil {
		t.Fatal("runtime disposal must detach the active permission session")
	}
}
