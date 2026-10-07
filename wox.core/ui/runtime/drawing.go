package woxui

import (
	window "wox/ui/runtime/internal/window"
)

// SupportsCaretPatch identifies renderers that can restore a saved caret-sized backdrop.
func SupportsCaretPatch() bool { return window.SupportsCaretPatch() }

// FontWeight names portable text weights without exposing platform numeric values.
type FontWeight = window.FontWeight

const (
	FontWeightRegular  = window.FontWeightRegular
	FontWeightSemibold = window.FontWeightSemibold
)

// FontFamily selects the portable text family category used by a draw command.
type FontFamily = window.FontFamily

const (
	FontFamilyUI        = window.FontFamilyUI
	FontFamilyMonospace = window.FontFamilyMonospace
)

// TextStyle describes portable font traits shared by measurement and drawing.
type TextStyle = window.TextStyle

// TextMetrics describes one shaped line in logical pixels.
type TextMetrics = window.TextMetrics

// DisplayList records the drawing commands for one frame.
type DisplayList = window.DisplayList

const (
	MaxConvexPolygonPoints     = window.MaxConvexPolygonPoints
	FloatingMaterialBlurMargin = window.FloatingMaterialBlurMargin
)

// FloatingMaterialStyle adjusts only the sampled backdrop; tint and alpha stay independent.
// Sigma is in logical units. Brightness and saturation are multipliers of the native result.
type FloatingMaterialStyle = window.FloatingMaterialStyle

// DefaultFloatingMaterialStyle delegates to its window implementation.
func DefaultFloatingMaterialStyle() FloatingMaterialStyle {
	return window.DefaultFloatingMaterialStyle()
}

// SoftwareRenderer is a deterministic retained RGBA target for damage correctness tests.
// It intentionally approximates platform text antialiasing while preserving command geometry,
// clipping, alpha compositing, and partial-frame retention.
type SoftwareRenderer = window.SoftwareRenderer

// NewSoftwareRenderer creates one persistent reference surface.
func NewSoftwareRenderer(width, height int) (*SoftwareRenderer, error) {
	return window.NewSoftwareRenderer(width, height)
}
