//go:build linux

package woxui

import "testing"

// A label's slot is measure_text's logical width, scaled. Drawing used to lay
// the same string out at the scaled font size with hinted advances, which round
// per glyph against the device grid instead of the logical one, so the drawn
// line ran past the slot it was given. With a Pango width set that wrapped the
// tail onto a second line the single-line surface discarded, painting
// "example-org/widget" as "example-org/"; without one it clips the last glyph.
// The drawn line has to stay on one line and inside its slot at every scale.
//
// The labels cover each break opportunity a tail can be lost at: a slash, a
// hyphen, a run of both, a space, and a label short enough that slot rounding
// alone used to hide the defect.
func TestLinuxDrawnTextFitsItsSlot(t *testing.T) {
	labels := []string{
		"example-org/widget",
		"acme-labs/metrics-agent",
		"https://example.com/acme-labs/toolkit",
		"Shared documentation index",
		"Open in browser",
		"Quick note",
	}
	scales := []float32{1, 1.25, 1.5, 2, 3}
	sizes := []float32{12, 15, 28}
	for _, label := range labels {
		for _, scale := range scales {
			for _, size := range sizes {
				overflow, lines := testLinuxDrawnTextFit(label, "", size, scale)
				if overflow == -2147483648 {
					t.Fatalf("native layout failed for %q at size %v scale %v", label, size, scale)
				}
				if lines != 1 {
					t.Errorf("%q at size %v scale %v wrapped onto %d lines; the tail would be dropped", label, size, scale, lines)
				}
				if overflow > 0 {
					t.Errorf("%q at size %v scale %v drew %dpx past its slot; the last glyph would be clipped", label, size, scale, overflow)
				}
			}
		}
	}
}
