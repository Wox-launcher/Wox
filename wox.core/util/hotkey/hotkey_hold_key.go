package hotkey

import (
	"fmt"
	"sync"
	"time"
	"wox/util"
	"wox/util/keyboard"
)

// standaloneHold buffers a key until it is known to be a tap or a hold.
type standaloneHold struct {
	mu           sync.Mutex
	key          keyboard.Key
	listener     keyboard.RawKeySubscription
	timer        *time.Timer
	sequence     uint64
	held         bool
	fired        bool
	replayed     bool
	closed       bool
	nativeCode   uint32
	target       uint64
	onPress      func()
	onRelease    func()
	onTransition func(HoldTransition)
	replay       func(uint32) error
	inputTarget  func() uint64
}

// startStandaloneHold installs an intercepting listener only where taps can be replayed.
func startStandaloneHold(key keyboard.Key, onPress, onRelease func(), transition func(HoldTransition)) (*standaloneHold, error) {
	if !keyboard.SupportsStandaloneHold(key) {
		return nil, fmt.Errorf("standalone hold hotkey is unavailable for %s on this platform", normalKeyString(key))
	}
	h := &standaloneHold{key: key, onPress: onPress, onRelease: onRelease, onTransition: transition, replay: keyboard.ReplayStandaloneTap, inputTarget: keyboard.ForegroundInputTarget}
	listener, err := addRawKeyListener(h.handle)
	if err != nil {
		return nil, err
	}
	if !keyboard.StandaloneHoldListenerReady() {
		_ = listener.Close()
		return nil, fmt.Errorf("standalone hold keyboard listener is unavailable")
	}
	h.listener = listener
	return h, nil
}

// handle consumes a standalone gesture while leaving modified shortcuts untouched.
func (h *standaloneHold) handle(event keyboard.RawKeyEvent) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return false
	}

	if event.Key != h.key {
		if h.held && event.Type == keyboard.EventTypeKeyDown {
			h.finishLocked(HoldEndInterrupted)
		}
		return false
	}
	if event.Type == keyboard.EventTypeKeyDown {
		if h.held {
			return true
		}
		if event.Modifiers != 0 {
			return false
		}
		h.held = true
		h.replayed = false
		h.nativeCode = event.NativeKeyCode
		if h.inputTarget != nil {
			h.target = h.inputTarget()
		}
		h.sequence++
		sequence := h.sequence
		h.timer = time.AfterFunc(holdPressDelay, func() {
			h.mu.Lock()
			defer h.mu.Unlock()
			if !h.held || h.replayed || sequence != h.sequence {
				return
			}
			h.timer = nil
			h.fired = true
			if h.onTransition != nil {
				h.onTransition(HoldTransition{Started: true})
			}
			if h.onPress != nil {
				util.Go(util.NewTraceContext(), "standalone hold press", h.onPress)
			}
		})
		return true
	}
	if event.Type == keyboard.EventTypeKeyUp && h.held {
		h.finishLocked(HoldEndReleased)
		h.held = false
		return true
	}
	return false
}

// finishLocked ends a hold or restores the buffered tap exactly once.
func (h *standaloneHold) finishLocked(reason HoldEndReason) {
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
	h.sequence++
	if h.fired {
		h.fired = false
		h.replayed = true
		if h.onTransition != nil {
			h.onTransition(HoldTransition{Reason: reason})
		}
		if h.onRelease != nil {
			util.Go(util.NewTraceContext(), "standalone hold release", h.onRelease)
		}
	} else if !h.replayed {
		h.replayed = true
		if h.inputTarget != nil && (h.target == 0 || h.target != h.inputTarget()) {
			util.GetLogger().Warn(util.NewTraceContext(), "standalone key tap was not replayed because focus changed")
			return
		}
		if err := h.replay(h.nativeCode); err != nil {
			util.GetLogger().Error(util.NewTraceContext(), fmt.Sprintf("failed to replay standalone key: %s", err))
		}
	}
}

// close ends an active gesture before removing the native listener.
func (h *standaloneHold) close() {
	h.mu.Lock()
	h.closed = true
	if h.held {
		h.finishLocked(HoldEndUnavailable)
		h.held = false
	}
	h.mu.Unlock()
	if h.listener != nil {
		_ = h.listener.Close()
	}
}
