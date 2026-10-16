package supervisor

import "time"

// hangObservation requires sustained failures and captures once until recovery.
// A long sampling gap (for example system sleep) starts a fresh observation.
type hangObservation struct {
	firstFailure time.Time
	lastSample   time.Time
	captured     bool
}

// observe returns true once a continuous stall reaches the capture threshold.
func (h *hangObservation) observe(now time.Time, unresponsive bool) bool {
	if !unresponsive || (!h.captured && !h.lastSample.IsZero() && now.Sub(h.lastSample) > 15*time.Second) {
		*h = hangObservation{}
	}
	h.lastSample = now
	if !unresponsive {
		return false
	}
	if h.firstFailure.IsZero() {
		h.firstFailure = now
	}
	if h.captured || now.Sub(h.firstFailure) < 30*time.Second {
		return false
	}
	h.captured = true
	return true
}
