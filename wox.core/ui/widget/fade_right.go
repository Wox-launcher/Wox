package widget

// FadeRight masks the right edge of its child without changing its layout.
// Width is the ramp length and Inset is the fully transparent trailing span.
type FadeRight struct {
	Width, Inset float32
	Child        Widget
}

func (w FadeRight) layout(ctx context, available constraints) *node {
	result := &node{fadeRight: w.Width, fadeRightInset: max(0, w.Inset)}
	if w.Child != nil {
		child := available.constrainNode(w.Child.layout(ctx, available))
		result.bounds.Width, result.bounds.Height = child.bounds.Width, child.bounds.Height
		result.children = []*node{child}
	}
	return available.constrainNode(result)
}
