//go:build !windows && !darwin && !linux

package woxui

func (w *platformWindow) setWindowChrome(bool, float32) error { return nil }
