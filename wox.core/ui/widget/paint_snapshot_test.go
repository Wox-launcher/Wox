package widget

import (
	"bytes"
	"testing"

	ui "wox/ui/runtime"
)

// TestSnapshotScopeDetachesInputAndPaint keeps only recorded paint after removing
// the live control. The overlay cannot retain focus, semantics or old callbacks.
func TestSnapshotScopeDetachesInputAndPaint(t *testing.T) {
	var snapshot *ui.PaintSnapshot
	live := true
	oldTaps, newTaps, paints := 0, 0, 0
	color := ui.Color{R: 255, A: 255}
	host := NewHost(func(ui.FrameInfo) Widget {
		if live {
			return SnapshotScope{Child: CapturePaint{OnCapture: func(value *ui.PaintSnapshot) { snapshot = value }, Child: Semantics{Key: "old", AutomationID: "old", Role: ui.AccessibilityRoleButton, Label: "Old", Child: Focusable{Key: "old", Child: Gesture{OnTap: func() { oldTaps++ }, Child: Painter{Width: 40, Height: 20,
				Paint: func(d *ui.DisplayList, bounds ui.Rect) { paints++; d.FillRect(bounds, color) }}}}}}}
		}
		return SnapshotScope{Child: Gesture{OnTap: func() { newTaps++ }, Child: Container{Width: 40, Height: 20}}, Layers: []Widget{SnapshotLayer{Snapshot: snapshot, Opacity: .5}}}
	})
	host.AttachServices(&fakeHostServices{})
	defer host.Dispose()
	renderTestFrame(host)
	if snapshot == nil || !host.FocusAutomationID("old") {
		t.Fatal("live source missing")
	}
	before := paints
	live, color = false, ui.Color{B: 255, A: 255}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		d := &ui.DisplayList{}
		host.Frame(d, ui.FrameInfo{Size: ui.Size{Width: 100, Height: 60}, Scale: scale})
		d.TransformScene(scale, ui.Point{})
		r, _ := ui.NewSoftwareRenderer(int(100*scale), int(60*scale))
		if err := r.Render(d); err != nil {
			t.Fatal(err)
		}
		if got := r.RGBA().RGBAAt(int(10*scale), int(10*scale)); got.R < 126 || got.R > 128 || got.B != 0 {
			t.Fatalf("snapshot called live paint or lost alpha: %+v", got)
		}
		if host.FocusAutomationID("old") {
			t.Fatal("snapshot retained focus target")
		}
		host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 10, Y: 10}})
		host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 10, Y: 10}})
	}
	if oldTaps != 0 || newTaps != 4 || paints != before {
		t.Fatalf("snapshot still interactive: old=%d new=%d paints=%d", oldTaps, newTaps, paints)
	}
}

// TestSnapshotScopeMovedOverflow covers changed root origins and a disappearing
// live child. Inherited group alpha stays live, while internal alpha stays frozen.
func TestSnapshotScopeMovedOverflow(t *testing.T) {
	var snapshot *ui.PaintSnapshot
	content := SnapshotScope{Child: Stack{Width: 80, Height: 60, Children: []StackChild{{Left: 20, Top: 10, Child: Opacity{Value: .5, Child: CapturePaint{OnCapture: func(value *ui.PaintSnapshot) { snapshot = value }, Child: Container{Width: 40, Height: 20, Color: ui.Color{R: 255, A: 255}}}}}}}}
	source := (Opacity{Value: .5, Child: content}).layout(context{}, constraints{width: 100, height: 80})
	source.place(-10, 15)
	source.draw(&ui.DisplayList{}, 0, 0, false, false, false, nil)
	if snapshot == nil || snapshot.Bounds() != (ui.Rect{X: 20, Y: 10, Width: 40, Height: 20}) {
		t.Fatalf("wrong local capture bounds: %+v", snapshot.Bounds())
	}
	for _, scale := range []float32{1, 1.5, 2} {
		matrix := ui.TranslationTransform(5, 4)
		scope := SnapshotScope{Layers: []Widget{SnapshotLayer{Snapshot: snapshot, Opacity: .5, Matrix: &matrix}}}
		n := (Opacity{Value: .5, Child: scope}).layout(context{}, constraints{width: 100, height: 80})
		n.place(15, 20)
		prepareNodeTree(n)
		if n.bounds.Width != 0 || n.bounds.Height != 0 {
			t.Fatal("snapshot occupies layout space")
		}
		if got := globalPaintBounds(n); got.X+got.Width < 80 || got.Y+got.Height < 54 {
			t.Fatalf("snapshot overflow missing from damage: %+v", got)
		}
		d := &ui.DisplayList{}
		n.draw(d, 0, 0, false, false, false, nil)
		d.TransformScene(scale, ui.Point{})
		r, _ := ui.NewSoftwareRenderer(int(100*scale), int(80*scale))
		if err := r.Render(d); err != nil {
			t.Fatal(err)
		}
		if got := r.RGBA().RGBAAt(int(50*scale), int(40*scale)); got.R < 31 || got.R > 33 {
			t.Fatalf("origin/group composition: %+v", got)
		}
		if got := r.RGBA().RGBAAt(int(20*scale), int(20*scale)); got.A != 0 {
			t.Fatal("snapshot moved to old origin")
		}
	}
}

type snapshotBoundaryProps struct {
	snapshot *ui.PaintSnapshot
	matrix   ui.AffineTransform
	opacity  float32
}

func (p snapshotBoundaryProps) Equal(other snapshotBoundaryProps) bool { return p == other }

// TestSnapshotBoundaryDamage compares retaining partial frames against complete
// repaint while snapshots move, fade, collapse, and are released from a cache.
func TestSnapshotBoundaryDamage(t *testing.T) {
	var snapshot *ui.PaintSnapshot
	n := (SnapshotScope{Child: CapturePaint{OnCapture: func(value *ui.PaintSnapshot) { snapshot = value }, Child: BoxShadow{Sigma: 3, Color: ui.Color{A: 100}, Child: Container{Width: 40, Height: 20, Color: ui.Color{R: 255, A: 255}}}}}).layout(context{}, constraints{width: 200, height: 120})
	n.draw(&ui.DisplayList{}, 0, 0, false, false, false, nil)
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		props := snapshotBoundaryProps{snapshot: snapshot, matrix: ui.IdentityTransform(), opacity: 1}
		host := NewHost(func(ui.FrameInfo) Widget {
			return Boundary[snapshotBoundaryProps]{Key: "snapshot", Props: props, Build: func(p snapshotBoundaryProps) Widget {
				return SnapshotScope{Child: Container{Width: 200, Height: 120, Color: ui.Color{B: 90, A: 255}}, Layers: []Widget{SnapshotLayer{Snapshot: p.snapshot, Opacity: p.opacity, Matrix: &p.matrix, OriginX: .5, OriginY: .5}}}
			}}
		})
		host.AttachServices(&fakeHostServices{})
		t.Cleanup(host.Dispose)
		retained, _ := ui.NewSoftwareRenderer(int(200*scale), int(120*scale))
		for step := range 5 {
			props.matrix = ui.TranslationTransform(float32(step)*20, float32(step)*10)
			props.opacity = 1 - float32(step)/5
			if step == 4 {
				props.snapshot = nil
			}
			frame := ui.FrameInfo{Size: ui.Size{Width: 200, Height: 120}, Scale: scale}
			if step > 0 {
				frame.Damage = ui.Rect{X: 1, Y: 1, Width: 1, Height: 1}
			}
			d := &ui.DisplayList{}
			host.Frame(d, frame)
			d.TransformScene(scale, ui.Point{})
			if err := retained.Render(d); err != nil {
				t.Fatal(err)
			}
			frame.Damage = ui.Rect{}
			fullList := &ui.DisplayList{}
			host.Frame(fullList, frame)
			fullList.TransformScene(scale, ui.Point{})
			full, _ := ui.NewSoftwareRenderer(int(200*scale), int(120*scale))
			if err := full.Render(fullList); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(retained.RGBA().Pix, full.RGBA().Pix) {
				t.Fatalf("scale=%g step=%d left stale snapshot pixels", scale, step)
			}
		}
	}
}
