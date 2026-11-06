package widget

import ui "wox/ui/runtime"

// PaintBackground paints only the container's decoration at logical bounds.
// Inline content can reuse box chrome without creating layout or input nodes;
// dimensions, padding and Child are deliberately ignored by both paint methods.
func (w Container) PaintBackground(list *ui.DisplayList, bounds ui.Rect) {
	if w.Floating {
		// Native material edges are one unit; other widths are drawn separately.
		edge := w.BorderColor
		if w.BorderWidth != 1 {
			edge = ui.Color{}
		}
		// Image-framed panels already supply their transparent silhouette.
		if w.Color.A != 0 || w.Surface == nil || w.Surface.Frame == nil {
			list.FloatingMaterial(bounds, w.Radius, w.Color, edge, w.Material)
		}
	} else if w.Color.A != 0 {
		list.FillRoundedRect(bounds, w.Radius, w.Color)
	}
	w.Surface.Paint(list, bounds, w.Radius)
	if w.BorderWidth > 0 && w.BorderColor.A != 0 && (!w.Floating || w.BorderWidth != 1) {
		list.StrokeRoundedRect(bounds, w.Radius, w.BorderWidth, w.BorderColor)
	}
	if !w.EdgeBordersOnTop {
		paintContainerEdgeBorders(list, bounds, w)
	}
}

// PaintForeground draws the decorations that must remain above opaque content.
func (w Container) PaintForeground(list *ui.DisplayList, bounds ui.Rect) {
	if w.Surface != nil && w.Surface.InnerShadow != nil {
		w.Surface.PaintOverlay(list, bounds)
	}
	if w.EdgeBordersOnTop {
		paintContainerEdgeBorders(list, bounds, w)
	}
}
