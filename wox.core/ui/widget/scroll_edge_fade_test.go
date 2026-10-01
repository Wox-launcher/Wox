package widget

import (
	"testing"
)

// TestScrollEdgeFadeTracksHiddenContent covers both ends, short documents and resizing.
func TestScrollEdgeFadeTracksHiddenContent(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		offset, height, content, top, bottom float32
	}{
		{"short", 0, 100, 60, 0, 0},
		{"collapsed", 0, 0, 300, 0, 0},
		{"top", 0, 100, 300, 0, 24},
		{"middle", 80, 100, 300, 24, 24},
		{"bottom", 200, 100, 300, 24, 0},
		{"near top", 3, 100, 300, 3, 24},
		{"near bottom", 198, 100, 300, 24, 2},
		{"expanded", 200, 400, 300, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := (ScrollView{Width: 100, Height: tc.height, ContentHeight: tc.content, Offset: tc.offset, EdgeFade: 24}).layout(context{window: &fakeHostServices{}}, constraints{width: 100, height: tc.height})
			if n.fadeTop != tc.top || n.fadeBottom != tc.bottom {
				t.Fatalf("fade=%v/%v want %v/%v", n.fadeTop, n.fadeBottom, tc.top, tc.bottom)
			}
		})
	}
}
