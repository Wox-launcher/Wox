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
		if h.observe(start.Add(time.Duration(i)*5*time.Second), true, false) {
			t.Fatal("captured a short stall")
		}
	}
	if !h.observe(start.Add(30*time.Second), true, false) {
		t.Fatal("missed sustained hang")
	}
	if h.observe(start.Add(time.Hour), true, false) {
		t.Fatal("captured the same hang again after long gap")
	}
	h.observe(start.Add(time.Hour+time.Second), false, false)
	if h.observe(start.Add(2*time.Hour), true, false) {
		t.Fatal("did not reset after recovery")
	}
	h.observe(start.Add(3*time.Hour), true, false)
	if h.observe(start.Add(3*time.Hour+5*time.Second), true, false) {
		t.Fatal("sleep counted towards timeout")
	}
}

// TestDebugHangObservation checks early capture, spaced repeats, recovery and sleep.
func TestDebugHangObservation(t *testing.T) {
	start := time.Now()
	var h hangObservation
	for i := 0; i <= 30; i++ {
		got := h.observe(start.Add(time.Duration(i)*time.Second), true, true)
		if got != (i == 5 || i == 15) {
			t.Fatalf("second %d: capture=%t", i, got)
		}
	}
	h.observe(start.Add(time.Minute), false, true)
	h.observe(start.Add(2*time.Minute), true, true)
	if h.observe(start.Add(3*time.Minute), true, true) {
		t.Fatal("sleep counted as a stall")
	}
	if !h.observe(start.Add(3*time.Minute+5*time.Second), true, true) {
		t.Fatal("missed hang after recovery")
	}
}
