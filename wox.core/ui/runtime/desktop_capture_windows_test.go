//go:build windows

package woxui

import (
	"image"
	"testing"
)

func TestCaptureWindowsRectRejectsEmptyBounds(t *testing.T) {
	if _, err := CaptureWindowsRect(image.Rect(0, 0, 0, 0)); err == nil {
		t.Fatal("empty capture rectangle should be rejected")
	}
}
