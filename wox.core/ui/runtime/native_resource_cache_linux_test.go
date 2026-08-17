//go:build linux

package woxui

import "testing"

func TestLinuxResourceCacheTracksGenerationAndRelease(t *testing.T) {
	if result := testLinuxResourceCacheGeneration(); result != 0 {
		t.Fatalf("Linux resource cache generation/release check failed, status=%d", result)
	}
}
