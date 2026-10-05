//go:build darwin

package woxui

import "testing"

// TestDarwinScreenshotWindowSelection runs the native logical-coordinate hit tests independently of Retina scale.
func TestDarwinScreenshotWindowSelection(t *testing.T) {
	if status := testDarwinScreenshotWindowSelection(); status != 0 {
		t.Fatalf("native screenshot window selection scenario %d failed", status)
	}
}
