package woxui

import "runtime"

// SupportsCaretPatch identifies renderers that can restore a saved caret-sized backdrop.
func SupportsCaretPatch() bool { return runtime.GOOS == "windows" }

// PrepareCaretPatch retains a complete scene and narrows damage only when the sole
// visual change is a caret blink. The native renderer falls back to this complete
// command stream if its small pixel cache was lost or could not be captured.
func (d *DisplayList) PrepareCaretPatch(previous *DisplayList, allowPatch bool) *DisplayList {
	d.caretCapture = 0
	d.caretPatch = Rect{}
	if d.damage.Width > 0 && d.damage.Height > 0 {
		return nil
	}
	for index, command := range d.commands {
		if command.kind == displayCommandBeginEmbeddedSurfaceOverlay {
			d.caretCapture = 0
			return nil
		}
		if command.caret {
			if d.caretCapture != 0 {
				d.caretCapture = 0
				return nil
			}
			d.caretCapture = index + 1
		}
	}
	if d.caretCapture == 0 {
		return nil
	}
	caret := d.commands[d.caretCapture-1]
	// Later paint must neither cover the saved pixels nor sample the blinking caret
	// through a floating blur. Rotated images use a conservative fallback.
	for _, command := range d.commands[d.caretCapture:] {
		if command.kind == displayCommandSetClipRect || command.kind == displayCommandClearClip {
			continue
		}
		if command.kind == displayCommandStrokeRoundedRect {
			// Window outlines are painted after their content. Their hollow interior
			// does not occlude the caret; retain a margin for stroke antialiasing.
			inset := command.stroke/2 + 4
			inner := Rect{X: command.rect.X + inset, Y: command.rect.Y + inset, Width: command.rect.Width - 2*inset, Height: command.rect.Height - 2*inset}
			radius := max(float32(0), min(command.radius, min(command.rect.Width, command.rect.Height)/2)-inset)
			if pointInRoundedRect(caret.rect.X, caret.rect.Y, inner, radius) &&
				pointInRoundedRect(caret.rect.X+caret.rect.Width, caret.rect.Y, inner, radius) &&
				pointInRoundedRect(caret.rect.X, caret.rect.Y+caret.rect.Height, inner, radius) &&
				pointInRoundedRect(caret.rect.X+caret.rect.Width, caret.rect.Y+caret.rect.Height, inner, radius) {
				continue
			}
		}
		margin := max(float32(4), command.stroke/2)
		if command.kind == displayCommandFloatingMaterial {
			margin += FloatingMaterialBlurMargin
		}
		bounds := Rect{X: command.rect.X - margin, Y: command.rect.Y - margin, Width: command.rect.Width + 2*margin, Height: command.rect.Height + 2*margin}
		intersection := intersectRects(caret.rect, bounds)
		if command.rotation != 0 || intersection.Width > 0 && intersection.Height > 0 {
			d.caretCapture = 0
			return nil
		}
	}
	unchanged := false
	if allowPatch && previous != nil && previous.clearColor == d.clearColor && len(previous.commands) == len(d.commands) {
		unchanged = true
		blink := false
		for index, command := range d.commands {
			old := previous.commands[index]
			if command.caret && old.caret {
				blink = command.caretVisible != old.caretVisible
				old.caretVisible = command.caretVisible
			}
			if !displayCommandsEqual(command, old) {
				unchanged = false
				break
			}
		}
		// Windows uses two retained swap-chain buffers. One unchanged frame must
		// replay the preceding damage before later blinks can touch only the caret.
		if unchanged && blink && previous.caretStable {
			d.caretPatch = caret.rect
			d.nativeDamage = caret.rect
		}
	}
	if previous == nil {
		previous = &DisplayList{}
	}
	previous.clearColor = d.clearColor
	previous.caretStable = unchanged
	clear(previous.commands)
	previous.commands = append(previous.commands[:0], d.commands...)
	return previous
}

// CaretPatch returns the validated logical blink region, or zero for normal painting.
func (d *DisplayList) CaretPatch() Rect { return d.caretPatch }
