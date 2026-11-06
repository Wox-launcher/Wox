package widget

import ui "wox/ui/runtime"

// Transform maps child paint around a relative origin (0,0 is top-left; .5,.5 is
// center). Layout, hit testing and accessibility retain the untransformed box.
// Matrix translations are logical units, independent of display scale. The zero
// matrix collapses paint; use ui.IdentityTransform for an unchanged child.
type Transform struct {
	Matrix           ui.AffineTransform
	OriginX, OriginY float32
	Child            Widget
}

// layout resolves the pivot after child measurement without adding layout space.
func (w Transform) layout(ctx context, available constraints) *node {
	result := &node{}
	if w.Child != nil {
		child := available.constrainNode(w.Child.layout(ctx, available))
		result.bounds.Width, result.bounds.Height = child.bounds.Width, child.bounds.Height
		result.children = []*node{child}
	}
	result = available.constrainNode(result)
	x, y := w.OriginX*result.bounds.Width, w.OriginY*result.bounds.Height
	matrix := ui.TranslationTransform(-x, -y).Then(w.Matrix).Then(ui.TranslationTransform(x, y))
	if matrix.Finite() && matrix != ui.IdentityTransform() {
		result.transform = &matrix
	}
	return result
}

// drawingTransform conjugates a local pivot into the window-space coordinates
// supplied to paint callbacks; the DisplayList composes enclosing transforms.
func (n *node) drawingTransform(bounds ui.Rect) ui.AffineTransform {
	if n.transform == nil {
		return ui.IdentityTransform()
	}
	return ui.TranslationTransform(-bounds.X, -bounds.Y).Then(*n.transform).Then(ui.TranslationTransform(bounds.X, bounds.Y))
}
