//go:build windows

package tooltip

import (
	"sync"
	"time"

	"wox/util/overlay"
)

type tracker struct {
	mu     sync.RWMutex
	opts   Options
	stopCh chan struct{}
}

var (
	trackersMu sync.Mutex
	trackers   = map[string]*tracker{}
)

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
	// Owner-window pointer-leave cannot dismiss the tooltip: showing this HWND
	// generates WM_MOUSELEAVE on the launcher/settings window. Poll until the
	// cursor leaves both the trigger and the overlay, or has already moved on
	// inside the owner window after the delayed show.
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
