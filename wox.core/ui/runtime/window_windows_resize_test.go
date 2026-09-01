//go:build windows

package woxui

import "testing"

func TestWindowsResizeNeedsPreparedFrame(t *testing.T) {
	tests := []struct {
		name                string
		width, height       int
		preparedFrameNeeded bool
	}{
		{name: "same size", width: 640, height: 360},
		{name: "grow height", width: 640, height: 480, preparedFrameNeeded: true},
		{name: "grow width", width: 800, height: 360, preparedFrameNeeded: true},
		{name: "grow both", width: 800, height: 480, preparedFrameNeeded: true},
		{name: "shrink height", width: 640, height: 240},
		{name: "shrink width", width: 480, height: 360},
		{name: "mixed dimensions", width: 800, height: 240},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := windowsResizeNeedsPreparedFrame(640, 360, test.width, test.height); got != test.preparedFrameNeeded {
				t.Fatalf("prepared frame needed = %t, want %t", got, test.preparedFrameNeeded)
			}
		})
	}
}
