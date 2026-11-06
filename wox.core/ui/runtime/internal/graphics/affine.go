package graphics

import "math"

// AffineTransform maps column vectors: x' = A*x + C*y + TX and
// y' = B*x + D*y + TY. Coordinates use the caller's units; only translations
// carry those units. The zero value collapses geometry, not the identity.
type AffineTransform struct {
	A, B, C, D float32
	TX, TY     float32
}

func IdentityTransform() AffineTransform { return AffineTransform{A: 1, D: 1} }

func TranslationTransform(x, y float32) AffineTransform {
	return AffineTransform{A: 1, D: 1, TX: x, TY: y}
}

func ScaleTransform(x, y float32) AffineTransform { return AffineTransform{A: x, D: y} }

// RotationTransform uses radians; positive angles turn clockwise in a top-left
// coordinate system with y increasing downward.
func RotationTransform(radians float32) AffineTransform {
	sine, cosine := math.Sincos(float64(radians))
	return AffineTransform{A: float32(cosine), B: float32(sine), C: -float32(sine), D: float32(cosine)}
}

// Then composes in application order: t.Then(next).Point(p) applies t first,
// followed by next. This keeps local, ancestor and physical-display maps explicit.
func (t AffineTransform) Then(next AffineTransform) AffineTransform {
	return AffineTransform{
		A:  float32(float64(next.A)*float64(t.A) + float64(next.C)*float64(t.B)),
		B:  float32(float64(next.B)*float64(t.A) + float64(next.D)*float64(t.B)),
		C:  float32(float64(next.A)*float64(t.C) + float64(next.C)*float64(t.D)),
		D:  float32(float64(next.B)*float64(t.C) + float64(next.D)*float64(t.D)),
		TX: float32(float64(next.A)*float64(t.TX) + float64(next.C)*float64(t.TY) + float64(next.TX)),
		TY: float32(float64(next.B)*float64(t.TX) + float64(next.D)*float64(t.TY) + float64(next.TY)),
	}
}

func (t AffineTransform) Point(p Point) Point {
	return Point{X: float32(float64(t.A)*float64(p.X) + float64(t.C)*float64(p.Y) + float64(t.TX)), Y: float32(float64(t.B)*float64(p.X) + float64(t.D)*float64(p.Y) + float64(t.TY))}
}

// Inverse rejects singular/nonfinite maps and unrepresentable results. A tiny
// valid scale is not rejected by an arbitrary determinant epsilon.
func (t AffineTransform) Inverse() (AffineTransform, bool) {
	if !t.Finite() {
		return AffineTransform{}, false
	}
	determinant := float64(t.A)*float64(t.D) - float64(t.B)*float64(t.C)
	if determinant == 0 {
		return AffineTransform{}, false
	}
	inverse := AffineTransform{
		A: float32(float64(t.D) / determinant), B: float32(-float64(t.B) / determinant),
		C: float32(-float64(t.C) / determinant), D: float32(float64(t.A) / determinant),
		TX: float32((float64(t.C)*float64(t.TY) - float64(t.D)*float64(t.TX)) / determinant),
		TY: float32((float64(t.B)*float64(t.TX) - float64(t.A)*float64(t.TY)) / determinant),
	}
	if !inverse.Finite() {
		return AffineTransform{}, false
	}
	return inverse, true
}

// Finite guards native APIs from NaN/Inf, including overflow after composition.
func (t AffineTransform) Finite() bool {
	for _, value := range [...]float32{t.A, t.B, t.C, t.D, t.TX, t.TY} {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return false
		}
	}
	return true
}

// RectPolygon preserves the exact transformed silhouette for clipping. Bounds
// alone is only a conservative paint/damage envelope for rotated rectangles.
func (t AffineTransform) RectPolygon(rect Rect) []Point {
	if rect.Width <= 0 || rect.Height <= 0 || !t.Finite() {
		return nil
	}
	points := []Point{t.Point(Point{X: rect.X, Y: rect.Y}), t.Point(Point{X: rect.X + rect.Width, Y: rect.Y}), t.Point(Point{X: rect.X + rect.Width, Y: rect.Y + rect.Height}), t.Point(Point{X: rect.X, Y: rect.Y + rect.Height})}
	for _, p := range points {
		if !finitePoint(p) {
			return nil
		}
	}
	return points
}

// Bounds encloses the four transformed corners without rounding to physical pixels.
func (t AffineTransform) Bounds(rect Rect) Rect { return PolygonBounds(t.RectPolygon(rect)) }

func finitePoint(p Point) bool {
	return !math.IsNaN(float64(p.X)) && !math.IsNaN(float64(p.Y)) && !math.IsInf(float64(p.X), 0) && !math.IsInf(float64(p.Y), 0)
}
