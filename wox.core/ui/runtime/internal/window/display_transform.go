package window

import "wox/ui/runtime/internal/graphics"

// displayClip keeps exact target geometry beside its cheap culling envelope.
// A nil polygon denotes an axis-aligned rectangle, including an empty clip.
type displayClip struct {
	rect   Rect
	points []Point
}

// PushTransform composes a local drawing map inside its parent. Translations use
// logical units; display scale is applied separately. Pair each call with PopTransform.
// Existing clips stay in target coordinates. Zero collapses paint; identity is explicit.
func (d *DisplayList) PushTransform(transform AffineTransform) {
	d.transformStack = append(d.transformStack, transform.Then(d.paintTransform()))
}

// PopTransform restores the previous drawing map without changing the clip stack.
func (d *DisplayList) PopTransform() {
	if len(d.transformStack) > 0 {
		d.transformStack = d.transformStack[:len(d.transformStack)-1]
	}
}

func (d *DisplayList) paintTransform() AffineTransform {
	if len(d.transformStack) == 0 {
		return IdentityTransform()
	}
	return d.transformStack[len(d.transformStack)-1]
}

func (d *DisplayList) appendClip(clip displayClip) {
	d.appendCommand(displayCommand{kind: displayCommandSetClipRect, rect: clip.rect, points: clip.points})
}

// transformedClip preserves the rectangle fast path until rotation/skew requires a polygon.
func (d *DisplayList) transformedClip(rect Rect) displayClip {
	transform := d.paintTransform()
	if float64(transform.A)*float64(transform.D)-float64(transform.B)*float64(transform.C) == 0 {
		// A diagonal collapsed rectangle can have a nonempty envelope but no area.
		return displayClip{}
	}
	clip := displayClip{rect: rect}
	if transform != IdentityTransform() {
		clip.rect = transform.Bounds(rect)
		if transform.B != 0 || transform.C != 0 {
			clip.points = transform.RectPolygon(rect)
		}
	}
	if len(d.clipStack) == 0 {
		return clip
	}
	parent := d.clipStack[len(d.clipStack)-1]
	if clip.points == nil && parent.points == nil {
		clip.rect = intersectRects(clip.rect, parent.rect)
		return clip
	}
	points, parentPoints := clip.points, parent.points
	if points == nil {
		points = IdentityTransform().RectPolygon(clip.rect)
	}
	if parentPoints == nil {
		parentPoints = IdentityTransform().RectPolygon(parent.rect)
	}
	clip.points = graphics.ClipConvexPolygon(points, parentPoints)
	clip.rect = graphics.PolygonBounds(clip.points)
	return clip
}

// displayTransformsEqual uses the same tolerance as other recorded geometry.
func displayTransformsEqual(a, b AffineTransform) bool {
	return displayListFloatsEqual(a.A, b.A) && displayListFloatsEqual(a.B, b.B) && displayListFloatsEqual(a.C, b.C) && displayListFloatsEqual(a.D, b.D) && displayListFloatsEqual(a.TX, b.TX) && displayListFloatsEqual(a.TY, b.TY)
}

// displayBackdropGroups marks just the groups whose pending content must be readable by a material.
func displayBackdropGroups(d *DisplayList) map[int]bool {
	var groups map[int]bool
	var stack []int
	for i, command := range d.commands {
		switch command.kind {
		case displayCommandBeginOpacity, displayCommandBeginEdgeFade:
			stack = append(stack, i)
		case displayCommandEndOpacity, displayCommandEndEdgeFade:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case displayCommandFloatingMaterial:
			if len(stack) > 0 && groups == nil {
				groups = make(map[int]bool)
			}
			for _, index := range stack {
				groups[index] = true
			}
		}
	}
	return groups
}
