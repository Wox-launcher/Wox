//go:build linux

package woxui

import "testing"

func TestLinuxBackgroundBlurSkipsScreenshotWindows(t *testing.T) {
	if testLinuxWindowRequestsBackgroundBlur(true, true) {
		t.Fatal("screenshot windows must keep showing the live desktop")
	}
	if !testLinuxWindowRequestsBackgroundBlur(false, true) {
		t.Fatal("launcher windows must request compositor blur when the protocol is available")
	}
	if testLinuxWindowRequestsBackgroundBlur(false, false) {
		t.Fatal("linux windows stay opaque when the compositor does not advertise blur")
	}
}
