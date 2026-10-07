//go:build linux

package window

import (
	"runtime"
	"testing"
)

func TestLinuxEdgeFadePreservesGlass(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	checkEdgeFadePixels(t, testLinuxEdgeFade)
}
