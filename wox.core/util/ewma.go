package util

import "sync"

// EWMA implements an Exponentially Weighted Moving Average.
// It is thread-safe and can be used to track latency metrics.
type EWMA struct {
	alpha float64
	value float64
	init  bool
	mu    sync.RWMutex
}

// NewEWMA creates a new EWMA with the given smoothing factor alpha.
// Alpha should be between 0 and 1. A smaller alpha means slower adaptation.
func NewEWMA(alpha float64) *EWMA {
	if alpha <= 0 || alpha > 1 {
		alpha = 0.2 // default
	}
	return &EWMA{
		alpha: alpha,
	}
}

// Add adds a new sample to the EWMA and returns the updated average.
func (e *EWMA) Add(sample float64) float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.init {
		e.value = sample
		e.init = true
	} else {
		e.value = e.alpha*sample + (1-e.alpha)*e.value
	}
	return e.value
}

// Value returns the current EWMA value.
// The second return value indicates whether any samples have been added.
func (e *EWMA) Value() (float64, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.value, e.init
}

// Reset clears the EWMA state.
func (e *EWMA) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.value = 0
	e.init = false
}
