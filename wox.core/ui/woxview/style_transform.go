package woxview

import (
	"math"

	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	"wox/ui/widget"
)

// transform wraps all node paint, including focus chrome, around its border-box
// center. Margin remains layout space and does not shift the rotation pivot.
func (s Style) transform(theme component.ControlTheme, child widget.Widget) widget.Widget {
	matrix := s.transformMatrix(theme)
	if matrix == ui.IdentityTransform() {
		return child
	}
	margin := s.Margin.resolve(theme)
	x, y := (margin.Left-margin.Right)/2, (margin.Top-margin.Bottom)/2
	matrix = ui.TranslationTransform(-x, -y).Then(matrix).Then(ui.TranslationTransform(x, y))
	return widget.Transform{Matrix: matrix, OriginX: .5, OriginY: .5, Child: child}
}

// transformMatrix shares local pose composition with inline fragment painting.
func (s Style) transformMatrix(theme component.ControlTheme) ui.AffineTransform {
	scale := float32(1)
	if s.Scale != nil {
		scale = *s.Scale
	}
	return ui.ScaleTransform(scale, scale).Then(ui.RotationTransform(s.Rotate * math.Pi / 180)).Then(ui.TranslationTransform(s.TranslateX.resolve(theme), s.TranslateY.resolve(theme)))
}
