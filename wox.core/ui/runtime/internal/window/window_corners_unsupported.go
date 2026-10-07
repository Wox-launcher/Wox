//go:build !windows && !darwin && !linux

package window

func (w *platformWindow) setWindowChrome(bool, float32, ...bool) error { return nil }
