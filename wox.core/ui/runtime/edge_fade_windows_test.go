//go:build windows

package woxui

import "testing"

func TestWindowsEdgeFadePreservesGlass(t *testing.T) {
	checkEdgeFadePixels(t, testWindowsEdgeFade)
}
