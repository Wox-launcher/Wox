package hotkey

import (
	"sync"
	"wox/util"
	"wox/util/keyboard"
)

type doubleTapState struct {
	isPressed         bool
	currentTapInvalid bool
	hasCompletedTap   bool
	lastTapAt         int64
}

type doubleTapTracker struct {
	mu     sync.Mutex
	states map[keyboard.Key]doubleTapState
}

func newDoubleTapTracker() *doubleTapTracker {
	return &doubleTapTracker{
		states: map[keyboard.Key]doubleTapState{},
	}
}

func (t *doubleTapTracker) Register(modifierKey keyboard.Key) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.states[modifierKey]; !exists {
		t.states[modifierKey] = doubleTapState{}
	}
}

func (t *doubleTapTracker) Unregister(modifierKey keyboard.Key) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.states, modifierKey)
}

func (t *doubleTapTracker) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return len(t.states)
}

func (t *doubleTapTracker) HandleEvent(event keyboard.RawKeyEvent, now int64) []keyboard.Key {
	if event.Key == keyboard.KeyUnknown {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	for modifierKey, state := range t.states {
		if modifierKey == event.Key {
			continue
		}

		// Any other key means the sequence is no longer a pure double tap.
		state.hasCompletedTap = false
		if state.isPressed {
			state.currentTapInvalid = true
		}
		t.states[modifierKey] = state
	}

	state, exists := t.states[event.Key]
	if !exists {
		return nil
	}

	switch event.Type {
	case keyboard.EventTypeKeyDown:
		if !state.isPressed {
			state.isPressed = true
			state.currentTapInvalid = false
		}
		t.states[event.Key] = state
		return nil
	case keyboard.EventTypeKeyUp:
		if !state.isPressed {
			// Ignore duplicate key-up events from the OS. A tap must start with a
			// fresh key-down, otherwise a single release can look like a double tap.
			return nil
		}

		state.isPressed = false
		if state.currentTapInvalid {
			state.currentTapInvalid = false
			state.hasCompletedTap = false
			t.states[event.Key] = state
			return nil
		}

		if state.hasCompletedTap && now-state.lastTapAt < 500 {
			state.hasCompletedTap = false
			t.states[event.Key] = state
			return []keyboard.Key{event.Key}
		}

		state.hasCompletedTap = true
		state.lastTapAt = now
		t.states[event.Key] = state
		return nil
	default:
		return nil
	}
}

var (
	doubleKeyCallbacks = util.NewHashMap[keyboard.Key, func()]()
	doubleKeyListener  keyboard.RawKeySubscription
	doubleKeyTracker   = newDoubleTapTracker()
)

func registerDoubleHotKey(modifierKey keyboard.Key, callback func()) error {
	doubleKeyCallbacks.Store(modifierKey, callback)
	doubleKeyTracker.Register(modifierKey)

	if doubleKeyListener != nil {
		return nil
	}

	listener, err := keyboard.AddRawKeyListener(func(event keyboard.RawKeyEvent) bool {
		triggeredKeys := doubleKeyTracker.HandleEvent(event, util.GetSystemTimestamp())
		for _, triggeredKey := range triggeredKeys {
			callback, ok := doubleKeyCallbacks.Load(triggeredKey)
			if !ok || callback == nil {
				continue
			}

			util.Go(util.NewTraceContext(), "double hotkey callback", func() {
				callback()
			})
		}

		return false
	})
	if err != nil {
		doubleKeyCallbacks.Delete(modifierKey)
		doubleKeyTracker.Unregister(modifierKey)
		return err
	}

	doubleKeyListener = listener
	return nil
}

func unregisterDoubleHotKey(modifierKey keyboard.Key) {
	doubleKeyCallbacks.Delete(modifierKey)
	doubleKeyTracker.Unregister(modifierKey)

	if doubleKeyCallbacks.Len() > 0 {
		return
	}

	if doubleKeyListener != nil {
		_ = doubleKeyListener.Close()
		doubleKeyListener = nil
	}
}
