package window

import (
	"math"
	"testing"
)

// opacityScene includes overlapping opaque siblings and a nested group over glass.
func opacityScene(opacity float32) *DisplayList {
	d := &DisplayList{}
	d.Clear(Color{B: 255, A: 128})
	bounds := Rect{Y: 8, Width: 96, Height: 80}
	d.BeginOpacity(bounds, opacity)
	d.FillRect(Rect{Y: 8, Width: 64, Height: 40}, Color{R: 255, A: 255})
	d.FillRect(Rect{X: 32, Y: 8, Width: 64, Height: 40}, Color{G: 255, A: 255})
	nested := Rect{Y: 60, Width: 96, Height: 20}
	d.BeginOpacity(nested, 0.5)
	d.FillRect(nested, Color{R: 255, G: 255, B: 255, A: 255})
	d.EndOpacity(nested, 0.5)
	d.EndOpacity(bounds, opacity)
	d.FillRect(Rect{Y: 90, Width: 96, Height: 6}, Color{R: 255, A: 255})
	return d
}

// checkOpacityPixels detects per-primitive alpha, double backdrop blending, and leaked layer state.
func checkOpacityPixels(t *testing.T, render func(float32, float32) ([]byte, int)) {
	t.Helper()
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, alpha := range []float32{0, 0.25, 0.5, 1} {
			pixels, status := render(scale, alpha)
			if status == 1 {
				t.Skip("native display unavailable")
			}
			if status != 0 {
				t.Fatalf("native opacity status=%d", status)
			}
			size := int(96 * scale)
			for _, sample := range []struct {
				x, y float32
				want [4]float32
			}{
				{16, 20, [4]float32{255 * alpha, 0, 128 * (1 - alpha), 128 + 127*alpha}},
				{48, 20, [4]float32{0, 255 * alpha, 128 * (1 - alpha), 128 + 127*alpha}},
				{80, 20, [4]float32{0, 255 * alpha, 128 * (1 - alpha), 128 + 127*alpha}},
				{48, 70, [4]float32{127.5 * alpha, 127.5 * alpha, 128 + 63.5*alpha, 128 + 63.5*alpha}},
				{48, 2, [4]float32{0, 0, 128, 128}},
				{48, 93, [4]float32{255, 0, 0, 255}},
			} {
				offset := (int(sample.y*scale)*size + int(sample.x*scale)) * 4
				for channel, want := range sample.want {
					if math.Abs(float64(float32(pixels[offset+channel])-want)) > 3 {
						t.Fatalf("scale %v opacity %v at %v,%v: %v, want %v", scale, alpha, sample.x, sample.y, pixels[offset:offset+4], sample.want)
					}
				}
			}
		}
	}
}

// TestSoftwareOpacityGroupPixels also ensures scene scaling never multiplies dimensionless alpha.
func TestSoftwareOpacityGroupPixels(t *testing.T) {
	checkOpacityPixels(t, func(scale, alpha float32) ([]byte, int) {
		d := opacityScene(alpha)
		d.TransformScene(scale, Point{})
		r, err := NewSoftwareRenderer(int(96*scale), int(96*scale))
		if err != nil {
			t.Fatal(err)
		}
		if err := r.Render(d); err != nil {
			t.Fatal(err)
		}
		return r.RGBA().Pix, 0
	})
}

// TestOpacityClipRestoration checks both entry and exit, then compares partial replay to a full frame.
func TestOpacityClipRestorationAndDamage(t *testing.T) {
	build := func(alpha float32) *DisplayList {
		d := &DisplayList{}
		d.Clear(Color{B: 255, A: 128})
		d.PushClipRect(Rect{X: 10, Y: 10, Width: 20, Height: 20})
		d.BeginOpacity(Rect{Width: 60, Height: 60}, alpha)
		d.FillRect(Rect{Width: 60, Height: 60}, Color{R: 255, A: 255})
		d.EndOpacity(Rect{Width: 60, Height: 60}, alpha)
		d.FillRect(Rect{X: 25, Y: 25, Width: 35, Height: 35}, Color{G: 255, A: 255})
		d.PopClipRect()
		return d
	}
	d := build(0.5)
	if d.commands[2].kind != displayCommandSetClipRect || d.commands[5].kind != displayCommandSetClipRect {
		t.Fatal("opacity did not restore native clip state")
	}
	r, _ := NewSoftwareRenderer(60, 60)
	if err := r.Render(d); err != nil {
		t.Fatal(err)
	}
	if c := r.RGBA().RGBAAt(40, 40); c.B != 128 || c.A != 128 || c.R != 0 || c.G != 0 {
		t.Fatalf("layer leaked through enclosing clip: %v", c)
	}
	next := build(0.25)
	next.SetDamage(Rect{X: 10, Y: 10, Width: 20, Height: 20})
	if err := r.Render(next); err != nil {
		t.Fatal(err)
	}
	full, _ := NewSoftwareRenderer(60, 60)
	if err := full.Render(build(0.25)); err != nil {
		t.Fatal(err)
	}
	if string(r.RGBA().Pix) != string(full.RGBA().Pix) {
		t.Fatal("damage-scoped opacity replay differs from a full frame")
	}
}

// TestOpacityAffectsComparisonAndDisablesCaretPatch keeps repaint shortcuts from bypassing composition.
func TestOpacityAffectsComparisonAndDisablesCaretPatch(t *testing.T) {
	if opacityScene(0.5).Compare(opacityScene(0.25)) == nil {
		t.Fatal("opacity change compared equal")
	}
	d := opacityScene(0.5)
	d.DrawCaret(Rect{X: 2, Y: 2, Width: 1, Height: 10}, Color{A: 255}, true)
	if d.PrepareCaretPatch(nil, true) != nil || d.caretCapture != 0 {
		t.Fatal("group compositing allowed a standalone caret patch")
	}
}
