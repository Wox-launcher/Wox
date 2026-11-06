package widget

import (
	"testing"

	ui "wox/ui/runtime"
)

// TestPaintOverlayKeepsGeometryAndInput checks foreground order, clipping, DPI,
// inherited opacity/transform, and the original child's hit and focus targets.
func TestPaintOverlayKeepsGeometryAndInput(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		paint := PaintOverlay{Child: Container{Width: 40, Height: 20, Color: ui.Color{B: 255, A: 255}}, Paint: func(d *ui.DisplayList, r ui.Rect) {
			d.FillRect(ui.Rect{X: r.X + 20, Y: r.Y - 5, Width: 40, Height: 40}, ui.Color{R: 255, A: 255})
		}}
		n := (Transform{Matrix: ui.TranslationTransform(10, 5), Child: Opacity{Value: .5, Child: paint}}).layout(context{}, constraints{width: 100, height: 60})
		if n.bounds.Width != 40 || n.bounds.Height != 20 {
			t.Fatal("overlay changed child size")
		}
		d := &ui.DisplayList{RasterScale: scale}
		n.drawAt(d, ui.Point{}, 0, 0, false, false, false, nil)
		d.TransformScene(scale, ui.Point{})
		r, _ := ui.NewSoftwareRenderer(int(100*scale), int(60*scale))
		if err := r.Render(d); err != nil {
			t.Fatal(err)
		}
		pixels := r.RGBA()
		if got := pixels.RGBAAt(int(20*scale), int(15*scale)); got.B < 126 || got.B > 128 || got.R != 0 {
			t.Fatalf("child paint %+v", got)
		}
		if got := pixels.RGBAAt(int(40*scale), int(15*scale)); got.R < 126 || got.R > 128 || got.B != 0 {
			t.Fatalf("foreground order %+v", got)
		}
		if got := pixels.RGBAAt(int(55*scale), int(15*scale)); got.A != 0 {
			t.Fatal("foreground escaped measured box")
		}
	}
	taps := 0
	host := NewHost(func(ui.FrameInfo) Widget {
		return PaintOverlay{Child: Semantics{Key: "target", AutomationID: "target", Role: ui.AccessibilityRoleButton, Label: "Target", Child: Focusable{Key: "target", Child: Gesture{ID: "target", OnTap: func() { taps++ }, Child: Container{Width: 40, Height: 20}}}}, Paint: func(d *ui.DisplayList, r ui.Rect) { d.FillRect(r, ui.Color{A: 255}) }}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	renderTestFrame(host)
	if !host.FocusAutomationID("target") {
		t.Fatal("foreground took focus")
	}
	host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 10, Y: 10}})
	host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 10, Y: 10}})
	if taps != 1 {
		t.Fatal("foreground intercepted input")
	}
}
