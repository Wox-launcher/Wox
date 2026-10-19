package supervisor

import "time"

// hangObservation requires sustained failures, ignoring long gaps such as system sleep.
// Debug sessions take two spaced snapshots to distinguish a wait from slow progress.
type hangObservation struct {
	firstFailure time.Time
	lastSample   time.Time
	lastCapture  time.Time
	captures     int
}

// observe returns whether a continuous stall warrants another capture attempt.
func (h *hangObservation) observe(now time.Time, unresponsive, debugMode bool) bool {
	threshold, sampleGap, limit := 30*time.Second, 15*time.Second, 1
	if debugMode {
		threshold, sampleGap, limit = 5*time.Second, 5*time.Second, 2
	}
	if !unresponsive || (h.captures == 0 && !h.lastSample.IsZero() && now.Sub(h.lastSample) > sampleGap) {
		*h = hangObservation{}
	}
	h.lastSample = now
	if !unresponsive {
		return false
	}
	if h.firstFailure.IsZero() {
		h.firstFailure = now
	}
	if h.captures >= limit || now.Sub(h.firstFailure) < threshold || (!h.lastCapture.IsZero() && now.Sub(h.lastCapture) < 10*time.Second) {
		return false
	}
	h.captures++
	h.lastCapture = now
	return true
}
