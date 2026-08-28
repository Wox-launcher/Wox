package tooltip

import (
	"sync"
	"time"

	"wox/util/overlay"
)

// tracker keeps the latest tooltip tracking bounds while the polling goroutine runs.
type tracker struct {
	mu     sync.RWMutex
	opts   Options
	stopCh chan struct{}
}

var (
	trackersMu sync.Mutex
	trackers   = map[string]*tracker{}
)

// startVisibilityTracking polls the OS cursor until it leaves the trigger and overlay.
// Showing a native tooltip window generates an owner-window leave, so widget
// pointer-leave cannot dismiss the hint. Linux also needs this path: glance
// leave is not an explicit dismiss, and Wayland cannot rely on that leave when
// the overlay sits above the trigger.
func startVisibilityTracking(opts Options) {
	if opts.Name == "" {
		return
	}

	trackersMu.Lock()
	current, exists := trackers[opts.Name]
	if !exists {
		current = &tracker{opts: opts, stopCh: make(chan struct{})}
		trackers[opts.Name] = current
		go current.run()
	} else {
		current.mu.Lock()
		current.opts = opts
		current.mu.Unlock()
	}
	trackersMu.Unlock()
}

// stopVisibilityTracking cancels cursor polling for a tooltip overlay.
func stopVisibilityTracking(name string) {
	trackersMu.Lock()
	current, exists := trackers[name]
	if exists {
		delete(trackers, name)
	}
	trackersMu.Unlock()
	if exists {
		close(current.stopCh)
	}
}

func (current *tracker) run() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	started := time.Now()
	seenInside := false
	if current.closeIfNeeded(&seenInside, started) {
		return
	}

	for {
		select {
		case <-current.stopCh:
			return
		case <-ticker.C:
			if current.closeIfNeeded(&seenInside, started) {
				return
			}
		}
	}
}

// closeIfNeeded dismisses the overlay once tracking decides the cursor has left.
func (current *tracker) closeIfNeeded(seenInside *bool, started time.Time) bool {
	current.mu.RLock()
	opts := current.opts
	current.mu.RUnlock()

	shouldClose, nextSeenInside := evaluateVisibility(opts, *seenInside, time.Since(started))
	*seenInside = nextSeenInside
	if !shouldClose {
		return false
	}

	stopVisibilityTracking(opts.Name)
	overlay.Close(opts.Name)
	return true
}
