package window

import (
	"bytes"
	stdimage "image"
	"image/color"
	"math"
	"testing"

	"wox/ui/runtime/internal/graphics"
)

// transformRecordedScene injects internal matrices to test encoders independently
// of the public recording stack. It leaves clip points in target coordinates.
func transformRecordedScene(d *DisplayList, transform AffineTransform) {
	for i := range d.commands {
		c := &d.commands[i]
		switch c.kind {
		case displayCommandSetClipRect:
			c.points = transform.RectPolygon(c.rect)
			c.rect = graphics.PolygonBounds(c.points)
		case displayCommandClearClip:
		default:
			c.transform, c.transformed = transform, true
		}
	}
}

// TestSoftwareTransformClipAndOpacity checks a rotated clip's actual silhouette,
// nested composition, following drawing, and physical scene conversion.
func TestSoftwareTransformClipAndOpacity(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		d := &DisplayList{}
		d.Clear(Color{B: 255, A: 255})
		d.PushClipRect(Rect{X: -10, Y: -10, Width: 20, Height: 20})
		d.BeginOpacity(Rect{X: -30, Y: -30, Width: 60, Height: 60}, 0.5)
		d.FillRect(Rect{X: -30, Y: -30, Width: 60, Height: 60}, Color{R: 255, A: 255})
		d.FillRect(Rect{Width: 30, Height: 30}, Color{G: 255, A: 255})
		d.EndOpacity(Rect{X: -30, Y: -30, Width: 60, Height: 60}, 0.5)
		d.PopClipRect()
		transform := RotationTransform(math.Pi / 4).Then(TranslationTransform(30, 30))
		transformRecordedScene(d, transform)
		d.FillRect(Rect{X: 50, Y: 50, Width: 8, Height: 8}, Color{R: 255, G: 255, A: 255})
		d.TransformScene(scale, Point{})
		r, _ := NewSoftwareRenderer(int(64*scale), int(64*scale))
		if err := r.Render(d); err != nil {
			t.Fatal(err)
		}
		pixels := r.RGBA()
		for _, sample := range []struct {
			x, y float32
			want color.RGBA
		}{
			{18, 18, color.RGBA{B: 255, A: 255}},
			{30, 23, color.RGBA{R: 127, B: 128, A: 255}},
			{30, 37, color.RGBA{G: 127, B: 128, A: 255}},
			{54, 54, color.RGBA{R: 255, G: 255, A: 255}},
		} {
			if got := pixels.RGBAAt(int(sample.x*scale), int(sample.y*scale)); got != sample.want {
				t.Fatalf("DPI %g at %g,%g: %+v want %+v", scale, sample.x, sample.y, got, sample.want)
			}
		}
	}
}

// TestSoftwareTransformImageAndText rotates vector, raster and reference glyph
// geometry together instead of moving only each command's bounding rectangle.
func TestSoftwareTransformImageAndText(t *testing.T) {
	source := stdimage.NewRGBA(stdimage.Rect(0, 0, 2, 1))
	source.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	source.SetRGBA(1, 0, color.RGBA{G: 255, A: 255})
	image, err := NewImage(source)
	if err != nil {
		t.Fatal(err)
	}
	d := &DisplayList{}
	d.DrawImage(image, Rect{Width: 20, Height: 10})
	d.DrawText("A", Rect{X: 25, Width: 10, Height: 10}, TextStyle{Size: 10}, Color{B: 255, A: 255})
	transformRecordedScene(d, RotationTransform(math.Pi/2).Then(TranslationTransform(30, 10)))
	r, _ := NewSoftwareRenderer(64, 64)
	if err := r.Render(d); err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		x, y int
		want color.RGBA
	}{{25, 15, color.RGBA{R: 255, A: 255}}, {25, 25, color.RGBA{G: 255, A: 255}}, {25, 37, color.RGBA{B: 255, A: 255}}, {15, 5, color.RGBA{}}} {
		if got := r.RGBA().RGBAAt(sample.x, sample.y); got != sample.want {
			t.Fatalf("pixel %d,%d: %+v", sample.x, sample.y, got)
		}
	}
}

// TestSoftwareTransformRetainedDamage compares partial replay to a fresh target
// after a transformed scene moves, then checks that a later frame resets state.
func TestSoftwareTransformRetainedDamage(t *testing.T) {
	build := func(x float32) *DisplayList {
		d := &DisplayList{}
		d.Clear(Color{B: 100, A: 255})
		d.FillRoundedRect(Rect{X: -8, Y: -8, Width: 16, Height: 16}, 3, Color{R: 255, A: 255})
		transformRecordedScene(d, ScaleTransform(1.5, 0.75).Then(RotationTransform(0.4)).Then(TranslationTransform(x, 30)))
		return d
	}
	retained, _ := NewSoftwareRenderer(80, 60)
	if err := retained.Render(build(25)); err != nil {
		t.Fatal(err)
	}
	partial := build(40)
	partial.SetDamage(Rect{X: 5, Y: 10, Width: 55, Height: 40})
	if err := retained.Render(partial); err != nil {
		t.Fatal(err)
	}
	fresh, _ := NewSoftwareRenderer(80, 60)
	if err := fresh.Render(build(40)); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(retained.RGBA().Pix, fresh.RGBA().Pix) {
		t.Fatal("partial damage differs from full replay")
	}
	plain := &DisplayList{}
	plain.FillRect(Rect{Width: 5, Height: 5}, Color{G: 255, A: 255})
	if err := retained.Render(plain); err != nil {
		t.Fatal(err)
	}
	if got := retained.RGBA().RGBAAt(2, 2); got != (color.RGBA{G: 255, A: 255}) {
		t.Fatalf("transform leaked to next frame: %+v", got)
	}
}

// TestSoftwareTransformCollapsedAndInvalid skips singular drawings, rejects
// nonfinite native coordinates, and never applies the caret backdrop shortcut.
func TestSoftwareTransformCollapsedAndInvalid(t *testing.T) {
	d := &DisplayList{}
	d.DrawCaret(Rect{Width: 2, Height: 20}, Color{R: 255, A: 255}, true)
	transformRecordedScene(d, ScaleTransform(0, 1))
	r, _ := NewSoftwareRenderer(40, 40)
	if err := r.Render(d); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r.RGBA().Pix, make([]byte, 40*40*4)) {
		t.Fatal("collapsed transform painted pixels")
	}
	if d.PrepareCaretPatch(nil, true) != nil || d.caretCapture != 0 {
		t.Fatal("transformed caret enabled backdrop patch")
	}
	d.commands[0].transform.A = float32(math.NaN())
	if r.Render(d) == nil {
		t.Fatal("nonfinite transform was accepted")
	}
}

// TestTransformSceneComposesAffineCoordinates keeps local command geometry
// unchanged while viewport conversion is appended exactly once to its matrix.
func TestTransformSceneComposesAffineCoordinates(t *testing.T) {
	d := &DisplayList{}
	box := Rect{X: 4, Y: 6, Width: 30, Height: 20}
	d.DrawText("text", box, TextStyle{Size: 14}, Color{A: 255})
	d.PushClipRect(box)
	local := RotationTransform(0.5).Then(TranslationTransform(20, -30))
	transformRecordedScene(d, local)
	d.TransformScene(1.5, Point{X: -40, Y: 20})
	d.TransformScene(0.5, Point{X: 2, Y: 3})
	want := local.Then(ScaleTransform(1.5, 1.5)).Then(TranslationTransform(-40, 20)).Then(ScaleTransform(0.5, 0.5)).Then(TranslationTransform(2, 3))
	if d.commands[0].rect != box || d.commands[0].style.Size != 14 || !displayTransformsEqual(d.commands[0].transform, want) {
		t.Fatalf("local geometry scaled twice: %+v", d.commands[0])
	}
	point := want.Point(Point{X: box.X, Y: box.Y})
	if !displayListFloatsEqual(d.commands[1].points[0].X, point.X) || !displayListFloatsEqual(d.commands[1].points[0].Y, point.Y) {
		t.Fatal("clip and content scene conversions diverged")
	}
	copy := &DisplayList{commands: append([]displayCommand(nil), d.commands...)}
	if err := d.Compare(copy); err != nil {
		t.Fatal(err)
	}
	copy.commands[0].transform.TX++
	if d.Compare(copy) == nil {
		t.Fatal("matrix change was invisible to scene comparison")
	}
}

// TestSoftwareTransformFadeAndNestedLayers keeps fades in local coordinates and
// restores an untransformed parent's alpha layer after a transformed child ends.
func TestSoftwareTransformFadeAndNestedLayers(t *testing.T) {
	for _, scale := range []float32{1, 1.5, 2} {
		fade := &DisplayList{}
		box := Rect{Width: 20, Height: 40}
		fade.BeginEdgeFade(box, 20, 0)
		fade.FillRect(box, Color{R: 255, G: 255, B: 255, A: 255})
		fade.EndEdgeFade(box, 20, 0)
		transform := RotationTransform(math.Pi / 2).Then(TranslationTransform(60, 10))
		transformRecordedScene(fade, transform)
		fade.TransformScene(scale, Point{})
		r, _ := NewSoftwareRenderer(int(80*scale), int(64*scale))
		if err := r.Render(fade); err != nil {
			t.Fatal(err)
		}
		inverse, _ := transform.Then(ScaleTransform(scale, scale)).Inverse()
		for _, x := range []float32{25, 45, 55} {
			px, py := int(x*scale), int(20*scale)
			local := inverse.Point(Point{X: float32(px) + 0.5, Y: float32(py) + 0.5})
			alpha := byte(255 * min(float32(1), max(float32(0), local.Y/20)))
			if got := r.RGBA().RGBAAt(px, py); got != (color.RGBA{R: alpha, G: alpha, B: alpha, A: alpha}) {
				t.Fatalf("DPI %g rotated fade=%+v want alpha %d", scale, got, alpha)
			}
		}
		inner := &DisplayList{}
		inner.BeginOpacity(box, 0.5)
		inner.FillRect(box, Color{G: 255, A: 255})
		inner.EndOpacity(box, 0.5)
		transformRecordedScene(inner, transform)
		outer := &DisplayList{}
		outer.BeginOpacity(Rect{Width: 80, Height: 64}, 0.5)
		outer.commands = append(outer.commands, inner.commands...)
		outer.EndOpacity(Rect{Width: 80, Height: 64}, 0.5)
		outer.TransformScene(scale, Point{})
		if err := r.Render(outer); err != nil {
			t.Fatal(err)
		}
		if got := r.RGBA().RGBAAt(int(40*scale), int(20*scale)); got != (color.RGBA{G: 63, A: 63}) {
			t.Fatalf("nested transformed alpha=%+v", got)
		}
	}
}
