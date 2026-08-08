package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxDialogBackdropDoesNotDismiss(t *testing.T) {
	cancelled := 0
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return WoxDialog(DialogProps{
			ID: "dialog", Width: 100, Height: 100, OverlayWidth: 200, OverlayHeight: 200,
			OnEscape: func() { cancelled++ }, Theme: Theme{}, Child: woxwidget.Container{Width: 100, Height: 100},
		})
	})
	host.AttachServices(&hotkeyRecorderHostServices{})
	defer host.Dispose()
	displayList := woxui.DisplayList{}
	host.Frame(&displayList, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 200}, PixelSize: woxui.PixelSize{Width: 200, Height: 200}, Scale: 1})

	clickHostAt(host, woxui.Point{X: 60, Y: 60})
	clickHostAt(host, woxui.Point{X: 10, Y: 10})
	if cancelled != 0 {
		t.Fatal("clicking the dialog surface dismissed through the backdrop")
	}
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) || cancelled != 1 {
		t.Fatalf("Escape cancelled = %d, want 1", cancelled)
	}
}

func TestWoxDialogEscapeOnlyClosesActiveModal(t *testing.T) {
	innerOpen := true
	innerCancelled := 0
	outerCancelled := 0
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		children := []woxwidget.StackChild{}
		if innerOpen {
			children = append(children, woxwidget.StackChild{Child: WoxDialog(DialogProps{
				ID: "inner", Width: 80, Height: 80, OverlayWidth: 200, OverlayHeight: 200, OnEscape: func() {
					innerOpen = false
					innerCancelled++
				}, Theme: Theme{}, Child: woxwidget.Container{Width: 80, Height: 80},
			})})
		}
		return WoxDialog(DialogProps{
			ID: "outer", Width: 140, Height: 140, OverlayWidth: 200, OverlayHeight: 200, OnEscape: func() { outerCancelled++ }, Theme: Theme{},
			Child: woxwidget.Stack{Width: 140, Height: 140, Children: children},
		})
	})
	host.AttachServices(&hotkeyRecorderHostServices{})
	defer host.Dispose()
	displayList := woxui.DisplayList{}
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 200}, PixelSize: woxui.PixelSize{Width: 200, Height: 200}, Scale: 1}
	host.Frame(&displayList, frame)

	if !host.Key(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) {
		t.Fatal("active modal did not consume Escape")
	}
	if innerCancelled != 1 || outerCancelled != 0 {
		t.Fatalf("cancel counts = inner %d, outer %d; want 1 and 0", innerCancelled, outerCancelled)
	}
	host.Frame(&displayList, frame)
	if !host.Key(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) || outerCancelled != 1 {
		t.Fatalf("outer cancel count = %d, want 1 after inner closes", outerCancelled)
	}
}

func clickHostAt(host *woxwidget.Host, point woxui.Point) {
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
}
