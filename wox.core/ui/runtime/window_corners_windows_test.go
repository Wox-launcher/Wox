//go:build windows

package woxui

import "testing"

// TestTrimmedRendererRetainsCornerClip verifies shape state can be replayed when GPU resources are recreated.
func TestTrimmedRendererRetainsCornerClip(t *testing.T) {
	r := &nativeRenderer{width: 800, height: 600}
	radius := float32(22.5)
	if err := r.setCornerRadius(&radius); err != nil {
		t.Fatal(err)
	}
	radius = 100
	if r.cornerRadius == nil || *r.cornerRadius != 22.5 || r.cornerClipSize != [2]int{800, 600} {
		t.Fatal("clip did not retain its own physical geometry")
	}
	r.width, r.height = 1000, 750
	if err := r.setCornerRadius(r.cornerRadius); err != nil {
		t.Fatal(err)
	}
	if r.cornerClipSize != [2]int{1000, 750} {
		t.Fatal("clip missed resized bounds")
	}
	zero := float32(0)
	if err := r.setCornerRadius(&zero); err != nil {
		t.Fatal(err)
	}
	if r.cornerRadius == nil || *r.cornerRadius != 0 {
		t.Fatal("square corners became a default shape")
	}
	if err := r.setCornerRadius(nil); err != nil {
		t.Fatal(err)
	}
	if r.cornerRadius != nil {
		t.Fatal("legacy theme kept the custom clip")
	}
}

func TestWindowCornerDiameterScalesAndClamps(t *testing.T) {
	for _, test := range []struct {
		radius, scale       float32
		width, height, want int32
	}{
		{18, 1, 800, 600, 36}, {18, 1.25, 1000, 750, 45}, {18, 2, 1600, 1200, 72},
		{1000, 1.5, 900, 90, 90}, {0, 2, 1600, 1200, 0},
	} {
		if got := windowCornerDiameter(test.radius, test.scale, test.width, test.height); got != test.want {
			t.Fatalf("%+v: got %d", test, got)
		}
	}
}
