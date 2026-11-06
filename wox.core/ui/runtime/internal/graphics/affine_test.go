package graphics

import (
	"math"
	"math/rand"
	"slices"
	"testing"
)

func nearPoint(a, b Point, tolerance float32) bool {
	return math.Abs(float64(a.X-b.X)) <= float64(tolerance) && math.Abs(float64(a.Y-b.Y)) <= float64(tolerance)
}

// TestAffineApplicationOrder catches swapped native matrix conventions and
// distinguishes logical translation from the physical display conversion.
func TestAffineApplicationOrder(t *testing.T) {
	local := ScaleTransform(2, 3).Then(RotationTransform(math.Pi / 2)).Then(TranslationTransform(-120, 40))
	if got := local.Point(Point{X: 4, Y: 5}); !nearPoint(got, Point{X: -135, Y: 48}, 0.0001) {
		t.Fatal(got)
	}
	for _, dpi := range []float32{1, 1.25, 1.5, 2} {
		physical := local.Then(ScaleTransform(dpi, dpi)).Then(TranslationTransform(-1920, 80))
		want := Point{X: -1920 - 135*dpi, Y: 80 + 48*dpi}
		if got := physical.Point(Point{X: 4, Y: 5}); !nearPoint(got, want, 0.001) {
			t.Fatalf("DPI %g: %+v want %+v", dpi, got, want)
		}
		inverse, ok := physical.Inverse()
		if !ok || !nearPoint(inverse.Point(want), Point{X: 4, Y: 5}, 0.001) {
			t.Fatalf("DPI %g inverse failed", dpi)
		}
	}
	if got := TranslationTransform(10, 20).Then(ScaleTransform(2, 3)).Point(Point{}); got != (Point{X: 20, Y: 60}) {
		t.Fatal("translation was not scaled", got)
	}
}

// TestAffineOriginAndBounds rotates around a fixed pivot while preserving the
// exact four-corner silhouette separately from its damage envelope.
func TestAffineOriginAndBounds(t *testing.T) {
	pivot := Point{X: 40, Y: 30}
	transform := TranslationTransform(-pivot.X, -pivot.Y).Then(RotationTransform(math.Pi / 2)).Then(TranslationTransform(pivot.X, pivot.Y))
	if got := transform.Point(pivot); !nearPoint(got, pivot, 0.0001) {
		t.Fatalf("pivot drifted: %+v", got)
	}
	box := transform.Bounds(Rect{X: 20, Y: 20, Width: 40, Height: 20})
	if !nearPoint(Point{box.X, box.Y}, Point{30, 10}, 0.0001) || !nearPoint(Point{box.Width, box.Height}, Point{20, 40}, 0.0001) {
		t.Fatalf("bounds=%+v", box)
	}
	reflected := ScaleTransform(-2, 3).Then(TranslationTransform(50, -20)).Bounds(Rect{X: 10, Y: 5, Width: 20, Height: 10})
	if reflected != (Rect{X: -10, Y: -5, Width: 40, Height: 30}) {
		t.Fatalf("reflected bounds=%+v", reflected)
	}
	if got := (AffineTransform{}).Bounds(Rect{Width: 10, Height: 20}); got.Width != 0 || got.Height != 0 {
		t.Fatal("zero transform was treated as identity")
	}
	if IdentityTransform().Bounds(Rect{Width: 0, Height: 10}) != (Rect{}) {
		t.Fatal("empty input acquired paint bounds")
	}
}

// TestAffineRoundTrip exercises general invertible affine maps, including skew,
// reflections and mixed-sign desktop positions using deterministic samples.
func TestAffineRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(73))
	for range 500 {
		value := func() float32 { return float32(rng.Float64()*4 - 2) }
		transform := AffineTransform{A: value(), B: value(), C: value(), D: value(), TX: value() * 500, TY: value() * 500}
		determinant := transform.A*transform.D - transform.B*transform.C
		if math.Abs(float64(determinant)) < 0.1 {
			continue
		}
		inverse, ok := transform.Inverse()
		if !ok {
			t.Fatal("invertible sample rejected")
		}
		point := Point{X: value() * 100, Y: value() * 100}
		if got := inverse.Point(transform.Point(point)); !nearPoint(got, point, 0.005) {
			t.Fatalf("round trip %+v -> %+v through %+v", point, got, transform)
		}
		if got := transform.Then(inverse).Point(point); !nearPoint(got, point, 0.005) {
			t.Fatalf("composition differs from sequential application: %+v", got)
		}
	}
}

// TestAffineInvalidMaps rejects bad native coordinates without rejecting valid
// tiny scales that merely have a small determinant.
func TestAffineInvalidMaps(t *testing.T) {
	for _, transform := range []AffineTransform{{}, ScaleTransform(0, 1), {A: 1, B: 2, C: 2, D: 4}, {A: float32(math.NaN()), D: 1}, {A: 1, D: 1, TX: float32(math.Inf(1))}, ScaleTransform(math.SmallestNonzeroFloat32, 1)} {
		if _, ok := transform.Inverse(); ok {
			t.Fatalf("invalid inverse accepted: %+v", transform)
		}
	}
	if inverse, ok := ScaleTransform(0.000001, 0.000001).Inverse(); !ok || !nearPoint(inverse.Point(Point{X: 0.000003, Y: 0.000004}), Point{X: 3, Y: 4}, 0.00001) {
		t.Fatal("tiny invertible scale rejected")
	}
	if ScaleTransform(math.MaxFloat32, math.MaxFloat32).Then(ScaleTransform(2, 2)).Finite() {
		t.Fatal("composition overflow was accepted")
	}
	if points := IdentityTransform().RectPolygon(Rect{X: math.MaxFloat32, Y: 0, Width: math.MaxFloat32, Height: 1}); points != nil {
		t.Fatal("corner overflow was accepted")
	}
	if got := PolygonBounds([]Point{{X: -math.MaxFloat32}, {X: math.MaxFloat32, Y: 1}}); got != (Rect{}) {
		t.Fatal("unrepresentable envelope was accepted")
	}
}

// TestConvexClipExactSilhouette proves a rotated clip is not its bounding box,
// and supports mirrored winding and non-100% scale at negative desktop origins.
func TestConvexClipExactSilhouette(t *testing.T) {
	for _, dpi := range []float32{1, 1.25, 1.5, 2} {
		transform := ScaleTransform(dpi, dpi).Then(TranslationTransform(-1920, 40))
		subject := transform.RectPolygon(Rect{Width: 10, Height: 10})
		clip := []Point{{X: 5, Y: -1}, {X: 11, Y: 5}, {X: 5, Y: 11}, {X: -1, Y: 5}}
		for i, p := range clip {
			clip[i] = transform.Point(p)
		}
		original := append([]Point(nil), subject...)
		for _, reverse := range []bool{false, true} {
			if reverse {
				slices.Reverse(clip)
			}
			result := ClipConvexPolygon(subject, clip)
			if len(result) != 8 || math.Abs(math.Abs(polygonArea(result))-68*float64(dpi*dpi)) > 0.001 {
				t.Fatalf("DPI %g winding %v: %v", dpi, reverse, result)
			}
			if !slices.Equal(subject, original) {
				t.Fatal("subject mutated")
			}
			corner := transform.RectPolygon(Rect{Width: 1, Height: 1})
			if len(ClipConvexPolygon(corner, result)) != 0 {
				t.Fatal("clip leaked into bounding-box corner")
			}
		}
	}
}

// TestConvexClipDegenerateAndDeep covers empty intersections and clip stacks
// whose exact intersection needs more than the renderer's fill-polygon limit.
func TestConvexClipDegenerateAndDeep(t *testing.T) {
	square := IdentityTransform().RectPolygon(Rect{Width: 10, Height: 10})
	for _, clip := range [][]Point{nil, {{}, {X: 1}, {X: 2}}, IdentityTransform().RectPolygon(Rect{X: 10, Width: 10, Height: 10}), IdentityTransform().RectPolygon(Rect{X: 20, Width: 10, Height: 10}), {{X: float32(math.NaN())}, {X: 1}, {Y: 1}}} {
		if len(ClipConvexPolygon(square, clip)) != 0 {
			t.Fatalf("empty intersection produced vertices: %v", clip)
		}
	}
	tiny := IdentityTransform().RectPolygon(Rect{X: 1e-8, Y: 1e-8, Width: 1e-8, Height: 1e-8})
	if len(ClipConvexPolygon(tiny, square)) != 4 {
		t.Fatal("finite subpixel clip discarded")
	}
	result := IdentityTransform().RectPolygon(Rect{X: -10, Y: -10, Width: 20, Height: 20})
	for i := 1; i < 64; i++ {
		clip := RotationTransform(float32(i) * math.Pi / 128).RectPolygon(Rect{X: -10, Y: -10, Width: 20, Height: 20})
		result = ClipConvexPolygon(result, clip)
	}
	area := math.Abs(polygonArea(result))
	if len(result) <= 16 || area < 314 || area > 315 {
		t.Fatalf("deep clip lost geometry: vertices=%d area=%g", len(result), area)
	}
}
