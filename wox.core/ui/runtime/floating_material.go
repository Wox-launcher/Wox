package woxui

import "errors"

// A floating material is one native translucent surface placed behind a panel
// that floats above other Go UI content in the same window, such as the
// launcher action panel. It samples the launcher pixels underneath, which the
// process-wide window material (see window_material.go) cannot do because that
// material sits behind the whole render view.
//
// Contract for callers: the panel must draw itself after
// DisplayList.BeginEmbeddedSurfaceOverlay so its pixels land on the overlay
// surface above the material, and it must not paint its own background there.
// The material owns the tint and the card edge: painting a wash from Go would
// sit above the material and hide the edge treatment it renders itself.
// Platforms without a per-region material accept the calls as no-ops, and the
// panel keeps painting its opaque background because HasFloatingMaterial is false.

// FloatingMaterialStyle describes how the material tints and outlines itself.
type FloatingMaterialStyle struct {
	CornerRadius float32
	// Tint is blended into the material, under whatever edge treatment it has.
	Tint Color
	// Edge is the hairline outlining the card. Even Liquid Glass needs it: its own
	// rim is a refraction of the backdrop and disappears over dark launcher content.
	Edge Color
}

// HasFloatingMaterial reports whether ShowFloatingMaterial produces a visible native material.
func HasFloatingMaterial() bool {
	return nativeFloatingMaterialAvailable()
}

// ShowFloatingMaterial places or moves the floating material, in logical client coordinates.
func (w *Window) ShowFloatingMaterial(bounds Rect, style FloatingMaterialStyle) error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return errors.New("floating material bounds must have a positive size")
	}
	style.CornerRadius = max(0, style.CornerRadius)
	return w.native.showFloatingMaterial(bounds, style)
}

// HideFloatingMaterial hides the floating material without discarding the native view.
func (w *Window) HideFloatingMaterial() error {
	if w == nil || w.native == nil {
		return errors.New("window is not initialized")
	}
	return w.native.hideFloatingMaterial()
}
