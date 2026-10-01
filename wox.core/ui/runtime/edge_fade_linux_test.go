//go:build linux

package woxui

import (
	"runtime"
	"testing"
)

func TestLinuxEdgeFadePreservesGlass(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	checkEdgeFadePixels(t, testLinuxEdgeFade)
}
