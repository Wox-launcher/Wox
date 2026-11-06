package widget

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	ui "wox/ui/runtime"
)

// TestTransformPaintAndOpacity checks pivots, independent sibling geometry and group overflow.
func TestTransformPaintAndOpacity(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		content := Stack{Width: 100, Height: 80, Children: []StackChild{
			{Left: 30, Top: 20, Child: Transform{Matrix: ui.RotationTransform(math.Pi / 2).Then(ui.TranslationTransform(15, -5)), OriginX: .5, OriginY: .5,
				Child: Container{Width: 20, Height: 10, Color: ui.Color{R: 255, A: 255}}}},
			{Left: 75, Top: 60, Child: Container{Width: 20, Height: 10, Color: ui.Color{G: 255, A: 255}}},
		}}
		n := (Opacity{Value: .5, Child: content}).layout(context{}, constraints{width: 100, height: 80})
		if n.bounds.Width != 100 || n.bounds.Height != 80 {
			t.Fatal("transform changed layout")
		}
		d := &ui.DisplayList{}
		d.Clear(ui.Color{B: 255, A: 255})
		n.draw(d, 0, 0, false, false, false, nil)
		d.TransformScene(scale, ui.Point{})
		r, _ := ui.NewSoftwareRenderer(int(100*scale), int(80*scale))
		if err := r.Render(d); err != nil {
			t.Fatal(err)
		}
		image := r.RGBA()
		if got := image.RGBAAt(int(55*scale), int(15*scale)); got.R < 126 || got.B < 126 || got.G != 0 {
			t.Fatalf("pivot/group at scale %g: %+v", scale, got)
		}
		if got := image.RGBAAt(int(32*scale), int(22*scale)); got.B != 255 || got.R != 0 {
			t.Fatalf("old position painted: %+v", got)
		}
		if got := image.RGBAAt(int(80*scale), int(65*scale)); got.G < 126 || got.R != 0 {
			t.Fatalf("sibling transformed: %+v", got)
		}
	}
	// A translated child extends beyond its layout-sized opacity parent.
	n := (Opacity{Value: .5, Child: Transform{Matrix: ui.TranslationTransform(40, 0), Child: Container{Width: 20, Height: 20, Color: ui.Color{R: 255, A: 255}}}}).layout(context{}, constraints{width: 100, height: 80})
	d := &ui.DisplayList{}
	n.draw(d, 0, 0, false, false, false, nil)
	r, _ := ui.NewSoftwareRenderer(100, 80)
	if err := r.Render(d); err != nil {
		t.Fatal(err)
	}
	if r.RGBA().RGBAAt(50, 10).R < 126 {
		t.Fatal("opacity clipped transformed overflow")
	}
}

type transformBoundaryProps struct{ builds *int }

func (p transformBoundaryProps) Equal(other transformBoundaryProps) bool { return p == other }

// TestTransformBoundaryDamage compares retained/culling frames with full repaint,
// including equal-envelope reflections, transform removal and cached subtree removal.
func TestTransformBoundaryDamage(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			matrix, present := ui.IdentityTransform(), true
			builds := 0
			host := NewHost(func(ui.FrameInfo) Widget {
				children := []StackChild{{Child: Container{Width: 200, Height: 120, Color: ui.Color{B: 90, A: 255}}}}
				if present {
					children = append(children, StackChild{Left: 60, Top: 40, Child: Transform{Matrix: matrix, OriginX: .5, OriginY: .5,
						Child: Boundary[transformBoundaryProps]{Key: "cached", Props: transformBoundaryProps{&builds}, Build: func(p transformBoundaryProps) Widget {
							*p.builds++
							return Flex{Axis: Horizontal, Children: []Widget{
								Container{Width: 20, Height: 20, Color: ui.Color{R: 255, A: 255}},
								Container{Width: 20, Height: 20, Color: ui.Color{G: 255, A: 255}},
							}}
						}}}})
				}
				return Stack{Width: 200, Height: 120, Children: children}
			})
			host.AttachServices(&fakeHostServices{})
			defer host.Dispose()
			retained, _ := ui.NewSoftwareRenderer(int(200*scale), int(120*scale))
			steps := []ui.AffineTransform{ui.IdentityTransform(), ui.ScaleTransform(-1, 1), ui.IdentityTransform(), ui.RotationTransform(math.Pi), ui.TranslationTransform(70, -25), {}, ui.IdentityTransform()}
			for step := 0; step <= len(steps); step++ {
				if step < len(steps) {
					matrix = steps[step]
				} else {
					present = false
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
				fullList := &ui.DisplayList{}
				frame.Damage = ui.Rect{}
				host.Frame(fullList, frame)
				fullList.TransformScene(scale, ui.Point{})
				full, _ := ui.NewSoftwareRenderer(int(200*scale), int(120*scale))
				if err := full.Render(fullList); err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(retained.RGBA().Pix, full.RGBA().Pix) {
					t.Fatalf("step %d left stale transformed pixels", step)
				}
			}
			if builds != 1 {
				t.Fatalf("transform invalidated Build cache: %d", builds)
			}
		})
	}
}

// TestTransformKeepsInputAndCaretGeometry distinguishes paint coordinates from layout/input.
func TestTransformKeepsInputAndCaretGeometry(t *testing.T) {
	taps := 0
	host := NewHost(func(ui.FrameInfo) Widget {
		return Transform{Matrix: ui.TranslationTransform(60, 20), Child: Focusable{Key: "field", Child: Gesture{OnTap: func() { taps++ },
			Child: Container{Width: 30, Height: 20, Color: ui.Color{R: 255, A: 255}}}}}
	})
	services := &fakeHostServices{}
	host.AttachServices(services)
	defer host.Dispose()
	for _, scale := range []float32{1, 1.5, 2, 1} {
		host.Frame(&ui.DisplayList{}, ui.FrameInfo{Size: ui.Size{Width: 120, Height: 80}, Scale: scale})
		if bounds, ok := host.BoundsForKey("field"); !ok || bounds != (ui.Rect{Width: 30, Height: 20}) {
			t.Fatalf("layout box moved: %+v", bounds)
		}
		host.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 10, Y: 10}})
		host.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Button: ui.PointerButtonPrimary, Position: ui.Point{X: 10, Y: 10}})
		if !host.InvalidateKey("field") || services.invalidatedRect != (ui.Rect{X: 60, Y: 20, Width: 30, Height: 20}) {
			t.Fatalf("wrong paint invalidation: %+v", services.invalidatedRect)
		}
	}
	if taps != 4 {
		t.Fatal("paint transform changed pointer routing")
	}
	n := (Transform{Matrix: ui.ScaleTransform(2, 2).Then(ui.TranslationTransform(20, 10)), Child: Container{Width: 10, Height: 10}}).layout(context{}, constraints{width: 100, height: 100})
	n.children[0].caret = true
	n.children[0].caretPaint = func(*ui.DisplayList, ui.Rect, bool, bool) {}
	prepareNodeTree(n)
	if got := activeCaretDamage(n, 0, false, false); got != (ui.Rect{X: 20, Y: 10, Width: 20, Height: 20}) {
		t.Fatalf("caret paint damage: %+v", got)
	}
}

// TestTransformAncestorClip keeps a negative-origin rotated shadow inside its
// untransformed viewport at fractional DPI; opacity must not bypass that clip.
func TestTransformAncestorClip(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		root := (Clip{SizeToChild: true, Child: Transform{Matrix: ui.RotationTransform(.6), OriginX: .5, OriginY: .5,
			Child: Opacity{Value: .5, Child: BoxShadow{Sigma: 4, Color: ui.Color{A: 128}, Child: Container{Width: 40, Height: 20, Color: ui.Color{R: 255, A: 255}}}}}}).layout(context{}, constraints{width: 100, height: 80})
		root.place(-10, 20)
		prepareNodeTree(root)
		d := &ui.DisplayList{}
		root.draw(d, 0, 0, false, false, false, nil)
		d.TransformScene(scale, ui.Point{})
		r, _ := ui.NewSoftwareRenderer(int(100*scale), int(80*scale))
		if err := r.Render(d); err != nil {
			t.Fatal(err)
		}
		pixels := r.RGBA()
		painted := 0
		for y := 0; y < pixels.Rect.Dy(); y++ {
			for x := 0; x < pixels.Rect.Dx(); x++ {
				if pixels.RGBAAt(x, y).A == 0 {
					continue
				}
				painted++
				if float32(x) >= 30*scale || float32(y) < 20*scale || float32(y) >= 40*scale {
					t.Fatalf("paint escaped clip at %d,%d scale %g", x, y, scale)
				}
			}
		}
		if painted < 100 {
			t.Fatal("transformed content disappeared")
		}
	}
}
