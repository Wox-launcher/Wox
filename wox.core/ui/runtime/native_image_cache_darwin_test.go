//go:build darwin

package woxui

import "testing"

func TestCachedCGImageOwnsNativePixels(t *testing.T) {
	if result := testCachedCGImageOwnsNativePixels(); result != 0 {
		t.Fatalf("cached CGImage still depended on the source buffer, status=%d", result)
	}
}

func TestLargeImageAdmissionRequiresCurrentCandidate(t *testing.T) {
	if result := testLargeImageAdmission(); result != 0 {
		t.Fatalf("large-image slot admission was not bound to the current candidate, status=%d", result)
	}
}
