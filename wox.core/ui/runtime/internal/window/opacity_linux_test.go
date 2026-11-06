//go:build linux

package window

import (
	"runtime"
	"testing"
)

func TestLinuxOpacityGroupPixels(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	checkOpacityPixels(t, testLinuxOpacity)
}
