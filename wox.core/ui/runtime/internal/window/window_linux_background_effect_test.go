//go:build linux

package window

import (
	"math"
	"testing"
)

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

// TestLinuxRoundedBlurRegion checks the exact native region, including tiny and resized surfaces.
// The protocol takes logical coordinates, so buffer scale and desktop origin cannot alter it.
func TestLinuxRoundedBlurRegion(t *testing.T) {
	for _, size := range [][2]int{{800, 500}, {101, 51}, {5, 5}, {1, 1}, {0, 0}} {
		for _, radius := range []float32{0, 8, 12.5, 999} {
			rects := testLinuxBlurRegion(size[0], size[1], radius)
			contains := func(x, y int) bool {
				for _, rect := range rects {
					if x >= rect[0] && x < rect[0]+rect[2] && y >= rect[1] && y < rect[1]+rect[3] {
						return true
					}
				}
				return false
			}
			for _, rect := range rects {
				if rect[0] < 0 || rect[1] < 0 || rect[0]+rect[2] > size[0] || rect[1]+rect[3] > size[1] {
					t.Fatalf("region outside surface %v: %v", size, rect)
				}
			}
			if size[0] == 0 {
				if len(rects) != 0 {
					t.Fatal("empty surface has blur")
				}
				continue
			}
			if radius == 0 && (!contains(0, 0) || !contains(size[0]-1, size[1]-1)) {
				t.Fatal("square blur lost corners")
			}
			if radius > 0 && size[0] > 4 && contains(0, 0) {
				t.Fatal("rounded blur leaked into transparent corner")
			}
			if size[0] > 4 && !contains(size[0]/2, size[1]/2) {
				t.Fatal("rounded blur lost center")
			}
			for y := 0; y < size[1]; y++ {
				for x := 0; x < size[0]; x++ {
					if contains(x, y) != contains(size[0]-x-1, size[1]-y-1) {
						t.Fatal("asymmetric blur region")
					}
				}
			}
		}
	}
}

// TestLinuxBlurRegionAcrossDisplays verifies protocol rectangles stay inside the
// painted physical outline at fractional scales and negative desktop origins.
func TestLinuxBlurRegionAcrossDisplays(t *testing.T) {
	for _, display := range []struct{ scale, x, y float64 }{
		{1, 0, 0}, {1.25, -1920, -400}, {1.5, 2560, -1080}, {2, -3840, 0},
	} {
		for _, width := range []int{101, 800} {
			const height = 51
			const radius = 8
			for _, rect := range testLinuxBlurRegion(width, height, radius) {
				for _, point := range [][2]int{{rect[0], rect[1]}, {rect[0] + rect[2], rect[1]}, {rect[0], rect[1] + rect[3]}, {rect[0] + rect[2], rect[1] + rect[3]}} {
					screenX := display.x + float64(point[0])*display.scale
					screenY := display.y + float64(point[1])*display.scale
					left, right := display.x+radius*display.scale, display.x+float64(width-radius)*display.scale
					top, bottom := display.y+radius*display.scale, display.y+(height-radius)*display.scale
					dx := math.Max(math.Max(left-screenX, screenX-right), 0)
					dy := math.Max(math.Max(top-screenY, screenY-bottom), 0)
					if math.Hypot(dx, dy) > radius*display.scale+0.0001 {
						t.Fatalf("blur outside physical outline: display=%+v rect=%v", display, rect)
					}
				}
			}
		}
	}
}
