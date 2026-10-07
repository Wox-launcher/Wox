//go:build !darwin && !linux

package woxui

const clipboardUsesEncodedPNG = false

func flushClipboard() error { return nil }
