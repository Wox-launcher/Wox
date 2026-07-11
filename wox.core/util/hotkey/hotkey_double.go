package hotkey

import (
	"fmt"
	"sync"
	"wox/util"
	"wox/util/keyboard"
)

type doublePressState struct {
	isPressed           bool
	currentPressInvalid bool
	hasCompletedPress   bool
	lastPressAt         int64
}

type doublePressTracker struct {
	mu     sync.Mutex
	states map[keyboard.Key]doublePressState
}

func newDoublePressTracker() *doublePressTracker {
	return &doublePressTracker{
		states: map[keyboard.Key]doublePressState{},
	}
}

func (t *doublePressTracker) Register(modifierKey keyboard.Key) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.states[modifierKey]; !exists {
		t.states[modifierKey] = doublePressState{}
	}
}

func (t *doublePressTracker) Unregister(modifierKey keyboard.Key) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.states, modifierKey)
}

func (t *doublePressTracker) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return len(t.states)
}

func (t *doublePressTracker) HandleEvent(event keyboard.RawKeyEvent, now int64) []keyboard.Key {
	if event.Key == keyboard.KeyUnknown {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	matchedModifierKeys := map[keyboard.Key]bool{}
	for modifierKey := range t.states {
		if modifierKeyMatchesRawEvent(modifierKey, event.Key) {
			matchedModifierKeys[modifierKey] = true
		}
	}

	for modifierKey, state := range t.states {
		if matchedModifierKeys[modifierKey] {
			continue
		}

		// Any other key means the sequence is no longer a pure double press.
		state.hasCompletedPress = false
		if state.isPressed {
			state.currentPressInvalid = true
		}
		t.states[modifierKey] = state
	}

	if len(matchedModifierKeys) == 0 {
		return nil
	}

	switch event.Type {
	case keyboard.EventTypeKeyDown:
		for modifierKey := range matchedModifierKeys {
			state := t.states[modifierKey]
			if !state.isPressed {
				state.isPressed = true
				state.currentPressInvalid = false
				util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf(
					"[double-press] key down: registered=%s event=%s, hasCompletedPress=%t", modifierKeyLogLabel(modifierKey), modifierKeyLogLabel(event.Key), state.hasCompletedPress))
			}
			t.states[modifierKey] = state
		}
		return nil
	case keyboard.EventTypeKeyUp:
		triggeredKeys := []keyboard.Key{}
		for modifierKey := range matchedModifierKeys {
			state := t.states[modifierKey]
			if !state.isPressed {
				// Ignore duplicate key-up events from the OS. A press must start with a
				// fresh key-down, otherwise a single release can look like a double press.
				continue
			}

			state.isPressed = false
			if state.currentPressInvalid {
				state.currentPressInvalid = false
				state.hasCompletedPress = false
				t.states[modifierKey] = state
				util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf(
					"[double-press] key up: registered=%s event=%s, press invalid (intervening key)", modifierKeyLogLabel(modifierKey), modifierKeyLogLabel(event.Key)))
				continue
			}

			if state.hasCompletedPress && now-state.lastPressAt < 500 {
				state.hasCompletedPress = false
				t.states[modifierKey] = state
				util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
					"[double-press] DOUBLE PRESS DETECTED: registered=%s event=%s, interval=%dms", modifierKeyLogLabel(modifierKey), modifierKeyLogLabel(event.Key), now-state.lastPressAt))
				cancelPressModifierPendingForDouble(event.Key)
				triggeredKeys = append(triggeredKeys, modifierKey)
				continue
			}

			state.hasCompletedPress = true
			state.lastPressAt = now
			t.states[modifierKey] = state
			util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf(
				"[double-press] key up: registered=%s event=%s, first press recorded, waiting for second press", modifierKeyLogLabel(modifierKey), modifierKeyLogLabel(event.Key)))
		}
		return triggeredKeys
	default:
		return nil
	}
}

var (
	doubleKeyCallbacks = util.NewHashMap[keyboard.Key, func()]()
	doubleKeyListener  keyboard.RawKeySubscription
	doubleKeyTracker   = newDoublePressTracker()
)

func registerDoubleHotKey(modifierKey keyboard.Key, callback func()) error {
	doubleKeyCallbacks.Store(modifierKey, callback)
	doubleKeyTracker.Register(modifierKey)

	if doubleKeyListener != nil {
		return nil
	}

	listener, err := addRawKeyListener(func(event keyboard.RawKeyEvent) bool {
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

func hasDoubleModifierRegistrationForRawKey(key keyboard.Key) bool {
	hasRegistration := false
	doubleKeyCallbacks.Range(func(registeredKey keyboard.Key, _ func()) bool {
		if modifierKeyMatchesRawEvent(registeredKey, key) {
			hasRegistration = true
			return false
		}
		return true
	})
	return hasRegistration
}
