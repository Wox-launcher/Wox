//go:build linux

package woxui

import "testing"

func TestLinuxResizeHitTestMatchesWindowsFramelessGrips(t *testing.T) {
	const (
		none      = int32(-1)
		northWest = int32(0)
		north     = int32(1)
		northEast = int32(2)
		west      = int32(3)
		east      = int32(4)
		southWest = int32(5)
		south     = int32(6)
		southEast = int32(7)
		width     = int32(300)
		height    = int32(200)
		grip      = int32(10)
	)
	tests := []struct {
		x, y float32
		want int32
	}{
		{x: 5, y: 5, want: northWest},
		{x: 295, y: 5, want: northEast},
		{x: 5, y: 195, want: southWest},
		{x: 295, y: 195, want: southEast},
		{x: 150, y: 5, want: north},
		{x: 5, y: 100, want: west},
		{x: 295, y: 100, want: east},
		{x: 150, y: 195, want: south},
		{x: 150, y: 100, want: none},
		{x: 11, y: 11, want: none},
	}
	for _, test := range tests {
		if got := testLinuxResizeHit(test.x, test.y, width, height, grip); got != test.want {
			t.Fatalf("resize hit at %.0f,%.0f = %d, want %d", test.x, test.y, got, test.want)
		}
	}
}
