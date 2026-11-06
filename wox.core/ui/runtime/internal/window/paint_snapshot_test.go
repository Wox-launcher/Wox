package window

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
	"testing"
)

// snapshotFragmentsFixture distinguishes shared ancestor alpha from equal but
// independent sibling groups while captures overlap in logical space.
func snapshotFragmentsFixture(d *DisplayList, shared bool, replay []*PaintSnapshot) []*PaintSnapshot {
	bounds := Rect{X: 10, Y: 10, Width: 60, Height: 40}
	d.BeginSnapshotScope(Point{})
	if replay != nil {
		d.DrawSnapshots(replay)
	} else {
		if shared {
			d.BeginOpacity(bounds, .5)
		}
		for i, region := range []Rect{bounds, {X: 25, Y: 15, Width: 25, Height: 20}} {
			if !shared {
				d.BeginOpacity(bounds, .5)
			}
			capture := d.BeginSnapshot(region)
			color := Color{B: 255, A: 255}
			if i == 1 {
				color = Color{R: 255, G: 255, B: 255, A: 255}
			}
			d.FillRect(region, color)
			replay = append(replay, d.EndSnapshot(capture))
			if !shared {
				d.EndOpacity(bounds, .5)
			}
		}
		if shared {
			d.EndOpacity(bounds, .5)
		}
	}
	d.EndSnapshotScope()
	d.FillRect(Rect{X: 75, Y: 15, Width: 10, Height: 10}, Color{G: 255, A: 255})
	return replay
}

// TestPaintSnapshotFragments prevents duplicated ancestor alpha and leaking
// fragment clips when a single logical node was captured in several paint calls.
func TestPaintSnapshotFragments(t *testing.T) {
	for _, shared := range []bool{true, false} {
		for _, scale := range []float32{1, 1.25, 1.5, 2} {
			t.Run(fmt.Sprintf("shared=%v/scale=%g", shared, scale), func(t *testing.T) {
				source, replay := &DisplayList{}, &DisplayList{}
				fragments := snapshotFragmentsFixture(source, shared, nil)
				snapshotFragmentsFixture(replay, shared, fragments)
				render := func(d *DisplayList) *image.RGBA {
					d.TransformScene(scale, Point{})
					r, _ := NewSoftwareRenderer(int(100*scale), int(70*scale))
					if err := r.Render(d); err != nil {
						t.Fatal(err)
					}
					return r.RGBA()
				}
				checkSnapshotPixels(t, render(replay), render(source))
			})
		}
	}
}

// snapshotFixture records child paint under internal clips/groups. External
// opacity and transforms are outside the scope and must not be captured twice.
func snapshotFixture(d *DisplayList, origin Point, replay *PaintSnapshot) *PaintSnapshot {
	d.PushTransform(TranslationTransform(-4, 3))
	outer := Rect{Width: 110, Height: 100}
	d.PushClipRect(outer)
	d.BeginOpacity(outer, .5)
	d.BeginSnapshotScope(origin)
	if replay != nil {
		d.DrawSnapshot(replay)
	} else {
		bounds := Rect{X: origin.X, Y: origin.Y, Width: 50, Height: 50}
		matrix := TranslationTransform(-origin.X-25, -origin.Y-25).Then(RotationTransform(math.Pi / 6)).Then(TranslationTransform(origin.X+25, origin.Y+25))
		d.PushTransform(matrix)
		d.PushClipRect(bounds)
		d.BeginOpacity(bounds, .5)
		d.BeginEdgeFade(bounds, 10, 10)
		capture := d.BeginSnapshot(bounds)
		d.FillRect(Rect{X: origin.X - 10, Y: origin.Y - 10, Width: 70, Height: 70}, Color{R: 255, A: 255})
		d.PushClipRect(Rect{X: origin.X + 5, Y: origin.Y + 5, Width: 20, Height: 25})
		d.BeginOpacity(bounds, .5)
		d.FillRect(bounds, Color{G: 255, A: 255})
		d.EndOpacity(bounds, .5)
		d.PopClipRect()
		pixels := image.NewRGBA(image.Rect(0, 0, 2, 2))
		pixels.SetRGBA(0, 0, color.RGBA{B: 255, A: 255})
		pixels.SetRGBA(1, 1, color.RGBA{R: 255, G: 255, A: 255})
		img, _ := NewImage(pixels)
		d.DrawImage(img, Rect{X: origin.X + 30, Y: origin.Y + 10, Width: 12, Height: 12})
		d.DrawText("Ab", Rect{X: origin.X + 8, Y: origin.Y + 27, Width: 30, Height: 16}, TextStyle{Size: 10}, Color{B: 255, A: 255})
		d.DrawCaret(Rect{X: origin.X + 12, Y: origin.Y + 8, Width: 2, Height: 16}, Color{B: 255, A: 255}, true)
		d.DrawCaret(Rect{X: origin.X + 16, Y: origin.Y + 8, Width: 2, Height: 16}, Color{B: 255, A: 255}, false)
		replay = d.EndSnapshot(capture)
		d.EndEdgeFade(bounds, 10, 10)
		d.EndOpacity(bounds, .5)
		d.PopClipRect()
		d.PopTransform()
	}
	d.EndSnapshotScope()
	d.EndOpacity(outer, .5)
	d.PopClipRect()
	d.PopTransform()
	// Following paint detects leaked clip, transform and group state.
	d.FillRect(Rect{X: 100, Y: 85, Width: 8, Height: 8}, Color{G: 255, A: 255})
	return replay
}

// checkSnapshotPixels allows one rounding unit when undoing/composing transforms.
func checkSnapshotPixels(t *testing.T, actual, expected *image.RGBA) {
	t.Helper()
	if actual.Rect != expected.Rect {
		t.Fatal("different raster dimensions")
	}
	for i, v := range actual.Pix {
		if delta := int(v) - int(expected.Pix[i]); delta < -1 || delta > 1 {
			t.Fatalf("snapshot pixel %d,%d channel %d: %d want %d", i/4%actual.Rect.Dx(), i/actual.Stride, i%4, v, expected.Pix[i])
		}
	}
}

// TestPaintSnapshotReplay compares full paint, including image/text/caret clips,
// at changed scope origins and physical scales, after detaching the source list.
func TestPaintSnapshotReplay(t *testing.T) {
	source := &DisplayList{}
	snapshot := snapshotFixture(source, Point{X: 15, Y: 12}, nil)
	if snapshot == nil || snapshot.RetainedBytes() <= 0 {
		t.Fatal("missing snapshot")
	}
	for i := range source.commands {
		source.commands[i] = displayCommand{}
	}
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		origin := Point{X: 25, Y: 22}
		actual, expected := &DisplayList{}, &DisplayList{}
		snapshotFixture(actual, origin, snapshot)
		snapshotFixture(expected, origin, nil)
		actual.TransformScene(scale, Point{})
		expected.TransformScene(scale, Point{})
		a, _ := NewSoftwareRenderer(int(120*scale), int(100*scale))
		e, _ := NewSoftwareRenderer(int(120*scale), int(100*scale))
		if err := a.Render(actual); err != nil {
			t.Fatal(err)
		}
		if err := e.Render(expected); err != nil {
			t.Fatal(err)
		}
		checkSnapshotPixels(t, a.RGBA(), e.RGBA())
		for _, c := range actual.commands {
			if c.caret {
				t.Fatal("snapshot retained live caret")
			}
		}
	}
}

// TestPaintSnapshotPartialCapture suspends only recording culls and keeps both
// nested captures complete. Hidden carets and single-use markers remain inert.
func TestPaintSnapshotPartialCapture(t *testing.T) {
	d := &DisplayList{}
	d.SetDamage(Rect{Width: 1, Height: 1})
	d.BeginSnapshotScope(Point{})
	outer := d.BeginSnapshot(Rect{Width: 60, Height: 20})
	d.FillRect(Rect{Width: 20, Height: 20}, Color{R: 255, A: 255})
	inner := d.BeginSnapshot(Rect{X: 40, Width: 20, Height: 20})
	d.FillRect(Rect{X: 40, Width: 20, Height: 20}, Color{G: 255, A: 255})
	i := d.EndSnapshot(inner)
	o := d.EndSnapshot(outer)
	if d.EndSnapshot(outer) != nil || d.snapshotDepth != 0 || len(i.commands) != 1 || len(o.commands) != 2 {
		t.Fatal("unbalanced capture")
	}
	d.EndSnapshotScope()
	d.FillRect(Rect{X: 80, Width: 20, Height: 20}, Color{B: 255, A: 255})
	if len(d.commands) != 2 {
		t.Fatal("capture left damage culling disabled")
	}
	replay := &DisplayList{}
	replay.BeginSnapshotScope(Point{})
	replay.DrawSnapshot(o)
	replay.EndSnapshotScope()
	r, _ := NewSoftwareRenderer(100, 30)
	if err := r.Render(replay); err != nil {
		t.Fatal(err)
	}
	if r.RGBA().RGBAAt(10, 10).R != 255 || r.RGBA().RGBAAt(50, 10).G != 255 {
		t.Fatal("partial capture omitted unchanged pixels")
	}
}

// TestPaintSnapshotParentClip enforces a newer rotated parent clip without
// changing the frozen snapshot, and verifies scope/group stacks stay balanced.
func TestPaintSnapshotParentClip(t *testing.T) {
	d := &DisplayList{}
	d.BeginSnapshotScope(Point{})
	c := d.BeginSnapshot(Rect{Width: 60, Height: 60})
	d.FillRect(Rect{Width: 60, Height: 60}, Color{R: 255, A: 255})
	snapshot := d.EndSnapshot(c)
	d.EndSnapshotScope()
	var first []byte
	for run := range 2 {
		replay := &DisplayList{}
		replay.BeginSnapshotScope(Point{})
		replay.PushTransform(RotationTransform(math.Pi / 4).Then(TranslationTransform(30, 20)))
		replay.PushClipRect(Rect{X: -10, Y: -10, Width: 20, Height: 20})
		replay.PopTransform()
		replay.DrawSnapshot(snapshot)
		clip, _ := replay.ClipRect()
		if clip.Width < 28 || clip.Width > 29 {
			t.Fatal("replay mutated portable clip")
		}
		replay.PopClipRect()
		replay.EndSnapshotScope()
		replay.FillRect(Rect{X: 60, Width: 10, Height: 10}, Color{G: 255, A: 255})
		r, _ := NewSoftwareRenderer(80, 60)
		if err := r.Render(replay); err != nil {
			t.Fatal(err)
		}
		if r.RGBA().RGBAAt(18, 8).A != 0 || r.RGBA().RGBAAt(30, 20).R != 255 || r.RGBA().RGBAAt(65, 5).G != 255 {
			t.Fatal("replay expanded or leaked clip")
		}
		if run == 0 {
			first = append([]byte(nil), r.RGBA().Pix...)
		} else if !bytes.Equal(first, r.RGBA().Pix) {
			t.Fatal("replay mutated snapshot")
		}
		if len(replay.groups) != 0 || len(replay.snapshotScopes) != 0 {
			t.Fatal("replay leaked scope")
		}
	}
}

// TestPaintSnapshotInvalidScope rejects collapsed parents and embedded surfaces
// while always restoring damage recording after a rejected capture.
func TestPaintSnapshotInvalidScope(t *testing.T) {
	d := &DisplayList{}
	if d.BeginSnapshot(Rect{}) != nil {
		t.Fatal("capture without scope")
	}
	d.PushTransform(ScaleTransform(0, 1))
	d.BeginSnapshotScope(Point{})
	if d.BeginSnapshot(Rect{}) != nil {
		t.Fatal("capture under singular parent")
	}
	d.EndSnapshotScope()
	d.PopTransform()
	d.BeginSnapshotScope(Point{})
	c := d.BeginSnapshot(Rect{Width: 10, Height: 10})
	d.appendCommand(displayCommand{kind: displayCommandBeginEmbeddedSurfaceOverlay})
	if d.EndSnapshot(c) != nil || d.snapshotDepth != 0 {
		t.Fatal("embedded surface retained")
	}
	d.EndSnapshotScope()
}
