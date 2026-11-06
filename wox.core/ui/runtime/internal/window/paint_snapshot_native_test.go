//go:build windows || darwin || linux

package window

import (
	"fmt"
	"image"
	"image/color"
	"testing"
)

// TestNativePaintSnapshotFragments exercises shared-group replay through the
// production backend at fractional and integer display scales.
func TestNativePaintSnapshotFragments(t *testing.T) {
	_, draw := nativeTransformTarget(t, 200, 140)
	for _, shared := range []bool{true, false} {
		for _, scale := range []float32{1, 1.25, 1.5, 2} {
			source, replay := &DisplayList{}, &DisplayList{}
			fragments := snapshotFragmentsFixture(source, shared, nil)
			snapshotFragmentsFixture(replay, shared, fragments)
			checkSnapshotPixels(t, draw(replay, scale), draw(source, scale))
		}
	}
}

// TestNativePaintSnapshotRegion compares the retained region with an explicit
// native clip, including antialiasing where its edge cuts through shared paint.
func TestNativePaintSnapshotRegion(t *testing.T) {
	_, draw := nativeTransformTarget(t, 200, 140)
	bounds := Rect{X: 10, Y: 10, Width: 60, Height: 40}
	region := Rect{X: 25, Y: 10, Width: 25, Height: 40}
	source := &DisplayList{}
	source.BeginSnapshotScope(Point{})
	capture := source.BeginSnapshotRegion(region)
	source.FillRoundedRect(bounds, 3, Color{B: 255, A: 255})
	snapshot := source.EndSnapshot(capture)
	source.EndSnapshotScope()
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		replay, expected := &DisplayList{}, &DisplayList{}
		replay.BeginSnapshotScope(Point{})
		replay.DrawSnapshot(snapshot)
		replay.EndSnapshotScope()
		expected.PushClipRect(region)
		expected.FillRoundedRect(bounds, 3, Color{B: 255, A: 255})
		expected.PopClipRect()
		checkSnapshotPixels(t, draw(replay, scale), draw(expected, scale))
	}
}

// TestNativePaintSnapshot sends captures through the production encoder, using
// the same scene at a new origin and display scale as the live reference tree.
func TestNativePaintSnapshot(t *testing.T) {
	_, draw := nativeTransformTarget(t, 240, 200)
	source := &DisplayList{}
	snapshot := snapshotFixture(source, Point{X: 15, Y: 12}, nil)
	for _, scale := range []float32{1, 1.25, 1.5, 2, 1} {
		for _, origin := range []Point{{X: 15, Y: 12}, {X: 25, Y: 22}} {
			t.Run(fmt.Sprintf("scale=%g/origin=%v", scale, origin), func(t *testing.T) {
				actual, expected := &DisplayList{}, &DisplayList{}
				snapshotFixture(actual, origin, snapshot)
				snapshotFixture(expected, origin, nil)
				checkSnapshotPixels(t, draw(actual, scale), draw(expected, scale))
			})
		}
	}
}

// TestNativeSnapshotMaterial keeps readable backdrop groups and sample metadata
// when a frozen command stream is replayed through a new scope origin.
func TestNativeSnapshotMaterial(t *testing.T) {
	_, draw := nativeTransformTarget(t, 192, 192)
	build := func(origin Point, snapshot *PaintSnapshot) (*DisplayList, *PaintSnapshot) {
		d := &DisplayList{}
		d.Clear(Color{R: 20, B: 100, A: 128})
		d.FillRect(Rect{X: 48, Width: 48, Height: 96}, Color{R: 255, G: 255, B: 255, A: 255})
		d.BeginSnapshotScope(origin)
		if snapshot != nil {
			d.DrawSnapshot(snapshot)
		} else {
			bounds := Rect{X: origin.X, Y: origin.Y, Width: 40, Height: 40}
			d.BeginOpacity(bounds, .5)
			d.PushClipRect(bounds)
			capture := d.BeginSnapshot(bounds)
			d.appendCommand(displayCommand{kind: displayCommandFloatingMaterial, rect: bounds, radius: 4, material: FloatingMaterialStyle{Sigma: 4, Brightness: 1, Saturation: 1}})
			snapshot = d.EndSnapshot(capture)
			d.PopClipRect()
			d.EndOpacity(bounds, .5)
		}
		d.EndSnapshotScope()
		return d, snapshot
	}
	_, snapshot := build(Point{X: 20, Y: 20}, nil)
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		actual, _ := build(Point{X: 30, Y: 30}, snapshot)
		expected, _ := build(Point{X: 30, Y: 30}, nil)
		if len(actual.floatingMaterials) != 1 || actual.floatingMaterials[0] != (Rect{X: 30, Y: 30, Width: 40, Height: 40}) || actual.materialMargin != 12 {
			t.Fatal("snapshot lost material damage metadata")
		}
		checkSnapshotPixels(t, draw(actual, scale), draw(expected, scale))
	}
}

// TestNativeImageCoordinateEquivalence prevents device snapping before an
// affine map from changing equivalent image draws at fractional display scales.
func TestNativeImageCoordinateEquivalence(t *testing.T) {
	_, draw := nativeTransformTarget(t, 192, 192)
	pixels := image.NewRGBA(image.Rect(0, 0, 2, 2))
	pixels.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	pixels.SetRGBA(1, 1, color.RGBA{G: 255, A: 255})
	img, _ := NewImage(pixels)
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, radians := range []float32{0, .4} {
			a, b := &DisplayList{}, &DisplayList{}
			matrix := RotationTransform(radians).Then(TranslationTransform(40, 30))
			a.PushTransform(matrix)
			a.DrawImage(img, Rect{X: 10, Y: 10, Width: 20, Height: 20})
			a.PopTransform()
			b.PushTransform(TranslationTransform(10, 10).Then(matrix))
			b.DrawImage(img, Rect{Width: 20, Height: 20})
			b.PopTransform()
			checkSnapshotPixels(t, draw(a, scale), draw(b, scale))
		}
	}
}
