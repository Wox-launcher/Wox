package window

import (
	"image/color"
	"math"
	"testing"
)

// recordedTransformScene combines an established parent clip with a rotated child clip.
func recordedTransformScene(d *DisplayList) {
	d.Clear(Color{B: 255, A: 255})
	d.PushTransform(TranslationTransform(40, 30))
	d.PushClipRect(Rect{X: -10, Y: -10, Width: 20, Height: 20})
	d.PushTransform(RotationTransform(math.Pi / 4))
	d.PushClipRect(Rect{X: -10, Y: -10, Width: 20, Height: 20})
	bounds := Rect{X: -40, Y: -40, Width: 80, Height: 80}
	d.BeginOpacity(bounds, .5)
	d.FillRect(bounds, Color{R: 255, A: 255})
	d.FillRect(Rect{Width: 40, Height: 40}, Color{G: 255, A: 255})
	d.EndOpacity(bounds, .5)
	d.PopClipRect()
	d.PopTransform()
	d.FillRect(Rect{X: -9, Y: -9, Width: 3, Height: 3}, Color{R: 255, G: 255, A: 255})
	d.PopClipRect()
	d.PopTransform()
	d.FillRect(Rect{X: 2, Y: 2, Width: 4, Height: 4}, Color{R: 255, B: 255, A: 255})
}

var recordedTransformSamples = []struct {
	x, y  float32
	color color.RGBA
}{
	{40, 24, color.RGBA{R: 128, B: 127, A: 255}},
	{40, 36, color.RGBA{G: 128, B: 127, A: 255}},
	{32, 22, color.RGBA{R: 255, G: 255, A: 255}},
	{48, 22, color.RGBA{B: 255, A: 255}},
	{25, 30, color.RGBA{B: 255, A: 255}},
	{3, 3, color.RGBA{R: 255, B: 255, A: 255}},
}

// TestRecordedTransformPixels verifies stack composition, clip restoration, scene scale and culling.
func TestRecordedTransformPixels(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, partial := range []bool{false, true} {
			d := &DisplayList{}
			if partial {
				d.SetDamage(Rect{X: 30, Y: 20, Width: 20, Height: 20})
			}
			recordedTransformScene(d)
			d.TransformScene(scale, Point{})
			r, _ := NewSoftwareRenderer(int(80*scale), int(60*scale))
			if err := r.Render(d); err != nil {
				t.Fatal(err)
			}
			for _, p := range recordedTransformSamples {
				if partial && (p.x < 30 || p.y < 20) {
					continue
				}
				got := r.RGBA().RGBAAt(int(p.x*scale), int(p.y*scale))
				for i, v := range []byte{got.R, got.G, got.B, got.A} {
					want := []byte{p.color.R, p.color.G, p.color.B, p.color.A}[i]
					if delta := int(v) - int(want); delta < -1 || delta > 1 {
						t.Fatalf("scale %g partial %v at %g,%g: %+v want %+v", scale, partial, p.x, p.y, got, p.color)
					}
				}
			}
		}
	}
}

// TestRecordedClipCoordinates ensures transform pops do not move established clips.
func TestRecordedClipCoordinates(t *testing.T) {
	d := &DisplayList{}
	d.PushTransform(TranslationTransform(20, 10))
	d.PushClipRect(Rect{Width: 10, Height: 10})
	if clip, ok := d.ClipRect(); !ok || clip != (Rect{Width: 10, Height: 10}) {
		t.Fatalf("local clip: %+v %v", clip, ok)
	}
	d.PopTransform()
	if clip, _ := d.ClipRect(); clip != (Rect{X: 20, Y: 10, Width: 10, Height: 10}) {
		t.Fatal(clip)
	}
	d.FillRect(Rect{Width: 50, Height: 50}, Color{R: 255, A: 255})
	d.PopClipRect()
	r, _ := NewSoftwareRenderer(50, 50)
	if err := r.Render(d); err != nil {
		t.Fatal(err)
	}
	if r.RGBA().RGBAAt(25, 15).R != 255 || r.RGBA().RGBAAt(5, 5).A != 0 {
		t.Fatal("clip moved with transform pop")
	}
	for i := 0; i < 64; i++ {
		d.PushTransform(RotationTransform(float32(i) * math.Pi / 128))
		d.PushClipRect(Rect{X: -10, Y: -10, Width: 20, Height: 20})
		d.PopTransform()
	}
	if got := len(d.clipStack[len(d.clipStack)-1].points); got < 32 {
		t.Fatalf("nested rotated clip lost vertices: %d", got)
	}
	for i := 0; i < 64; i++ {
		d.PopClipRect()
	}
	if _, ok := d.ClipRect(); ok {
		t.Fatal("clip stack did not unwind")
	}
}

// TestRecordedInvalidTransform is observable even when damage culling is enabled.
func TestRecordedInvalidTransform(t *testing.T) {
	d := &DisplayList{}
	d.SetDamage(Rect{Width: 10, Height: 10})
	d.PushTransform(AffineTransform{A: float32(math.NaN()), D: 1})
	d.FillRect(Rect{Width: 5, Height: 5}, Color{A: 255})
	d.PopTransform()
	r, _ := NewSoftwareRenderer(10, 10)
	if err := r.Render(d); err == nil {
		t.Fatal("nonfinite transform was silently culled")
	}
}

// TestRecordedCollapsedClip remains empty after returning to ordinary paint axes.
func TestRecordedCollapsedClip(t *testing.T) {
	for _, matrix := range []AffineTransform{{}, ScaleTransform(0, 1), {A: 1, B: 1, C: 1, D: 1}} {
		d := &DisplayList{}
		d.PushTransform(matrix)
		d.PushClipRect(Rect{Width: 20, Height: 20})
		d.PopTransform()
		d.FillRect(Rect{Width: 60, Height: 60}, Color{R: 255, A: 255})
		d.PopClipRect()
		r, _ := NewSoftwareRenderer(60, 60)
		if err := r.Render(d); err != nil {
			t.Fatal(err)
		}
		for _, v := range r.RGBA().Pix {
			if v != 0 {
				t.Fatal("collapsed clip admitted pixels")
			}
		}
	}
}

// TestRecordedMaterialBounds follows transformed backdrops when later damage is classified.
func TestRecordedMaterialBounds(t *testing.T) {
	if nativeFloatingMaterialMode() != floatingMaterialRendered {
		t.Skip("platform uses painted materials")
	}
	d := &DisplayList{}
	d.PushTransform(RotationTransform(math.Pi / 2).Then(TranslationTransform(50, 20)))
	d.FloatingMaterial(Rect{Width: 20, Height: 10}, 0, Color{}, Color{}, &FloatingMaterialStyle{Sigma: 2, Brightness: 1, Saturation: 1})
	d.PopTransform()
	if got := d.RenderedFloatingMaterialRects(); len(got) != 1 || !displayListRectsEqual(got[0], Rect{X: 40, Y: 20, Width: 10, Height: 20}) {
		t.Fatalf("material target bounds: %+v", got)
	}
	d.TransformScene(1.5, Point{X: -10, Y: 5})
	if got := d.RenderedFloatingMaterialRects()[0]; !displayListRectsEqual(got, Rect{X: 50, Y: 35, Width: 15, Height: 30}) {
		t.Fatalf("material scene bounds: %+v", got)
	}
	if d.FloatingMaterialSampleMargin() != 9 {
		t.Fatal("material halo missed scene scale")
	}
}
