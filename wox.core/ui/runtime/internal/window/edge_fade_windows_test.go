//go:build windows

package window

import "testing"

func TestWindowsEdgeFadePreservesGlass(t *testing.T) {
	checkEdgeFadePixels(t, testWindowsEdgeFade)
}
