//go:build windows

package window

import "testing"

func TestWindowsOpacityGroupPixels(t *testing.T) {
	checkOpacityPixels(t, testWindowsOpacity)
}
