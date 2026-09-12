//go:build linux

package woxui

// Linux blur keeps rectangular native regions; the portable painter applies supported corners.
func (w *platformWindow) setCornerRadius(radius float32) error { return nil }
