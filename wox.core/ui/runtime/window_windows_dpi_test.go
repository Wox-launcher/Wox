//go:build windows

package woxui

import (
	"testing"
	"unsafe"

	"github.com/lxn/win"
)

func TestWindowsSuggestedDPIBounds(t *testing.T) {
	tests := []struct {
		name   string
		bounds win.RECT
		valid  bool
	}{
		{name: "valid", bounds: win.RECT{Left: 100, Top: 200, Right: 900, Bottom: 700}, valid: true},
		{name: "zero width", bounds: win.RECT{Left: 100, Top: 200, Right: 100, Bottom: 700}},
		{name: "zero height", bounds: win.RECT{Left: 100, Top: 200, Right: 900, Bottom: 200}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, valid := windowsSuggestedDPIBounds(uintptr(unsafe.Pointer(&test.bounds)))
			if valid != test.valid {
				t.Fatalf("valid = %t, want %t", valid, test.valid)
			}
			if got != test.bounds {
				t.Fatalf("bounds = %+v, want %+v", got, test.bounds)
			}
		})
	}

	if _, valid := windowsSuggestedDPIBounds(0); valid {
		t.Fatal("nil suggested bounds should be invalid")
	}
}
