//go:build windows

package tooltip

import (
	"sync"
	"time"

	"wox/util/mouse"
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

func tooltipFontSizePt() float64 {
	return tooltipBaseFontSizePt
}

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
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-current.stopCh:
			return
		case <-ticker.C:
			current.mu.RLock()
			opts := current.opts
			current.mu.RUnlock()

			inside, ok := isCursorInsideTooltipOrAnchor(opts)
			if !ok || inside {
				continue
			}

			stopVisibilityTracking(opts.Name)
			overlay.Close(opts.Name)
			return
		}
	}
}

func isCursorInsideTooltipOrAnchor(opts Options) (bool, bool) {
	point, ok := mouse.CurrentPosition()
	if !ok {
		return false, false
	}

	return rectContains(point.X, point.Y, opts.AnchorX, opts.AnchorY, opts.AnchorWidth, opts.AnchorHeight) ||
		rectContains(point.X, point.Y, opts.X, opts.Y, opts.TooltipWidth, opts.TooltipHeight), true
}

func rectContains(cursorX float64, cursorY float64, left float64, top float64, width float64, height float64) bool {
	if width <= 0 || height <= 0 {
		return false
	}
	return cursorX >= left && cursorX < left+width && cursorY >= top && cursorY < top+height
}
