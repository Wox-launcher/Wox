package supervisor

import (
	"testing"
	"time"
)

// TestHangObservation covers transient stalls, sustained hangs, sleep and recovery.
func TestHangObservation(t *testing.T) {
	start := time.Now()
	var h hangObservation
	for i := 0; i < 6; i++ {
		if h.observe(start.Add(time.Duration(i)*5*time.Second), true) {
			t.Fatal("captured a short stall")
		}
	}
	if !h.observe(start.Add(30*time.Second), true) {
		t.Fatal("missed sustained hang")
	}
	if h.observe(start.Add(time.Hour), true) {
		t.Fatal("captured the same hang again after long gap")
	}
	h.observe(start.Add(time.Hour+time.Second), false)
	if h.observe(start.Add(2*time.Hour), true) {
		t.Fatal("did not reset after recovery")
	}
	h.observe(start.Add(3*time.Hour), true)
	if h.observe(start.Add(3*time.Hour+5*time.Second), true) {
		t.Fatal("sleep counted towards timeout")
	}
}
