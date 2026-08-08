package woxui

type bufferDamageHistory struct {
	previous     Rect
	previousFull bool
	valid        bool
}

// accumulate returns damage for a two-buffer retained surface and remembers the current frame.
func (h *bufferDamageHistory) accumulate(current Rect, currentFull bool) Rect {
	result := Rect{}
	if h.valid && !currentFull && !h.previousFull {
		result = unionRects(current, h.previous)
	}
	h.previous = current
	h.previousFull = currentFull
	h.valid = true
	return result
}

func (h *bufferDamageHistory) reset() {
	*h = bufferDamageHistory{}
}

func unionRects(left, right Rect) Rect {
	if left.Width <= 0 || left.Height <= 0 {
		return right
	}
	if right.Width <= 0 || right.Height <= 0 {
		return left
	}
	x := min(left.X, right.X)
	y := min(left.Y, right.Y)
	rightEdge := max(left.X+left.Width, right.X+right.Width)
	bottomEdge := max(left.Y+left.Height, right.Y+right.Height)
	return Rect{X: x, Y: y, Width: rightEdge - x, Height: bottomEdge - y}
}
