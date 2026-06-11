package hotkey

import (
	"fmt"
	"sync"
	"wox/util"
	"wox/util/keyboard"
)

type hyperKeyTracker struct {
	capsPressed           bool
	comboTriggered        bool
	passthroughCapsEvents int
	pressedKeys           map[keyboard.Key]bool
}

func newHyperKeyTracker() *hyperKeyTracker {
	return &hyperKeyTracker{
		pressedKeys: map[keyboard.Key]bool{},
	}
}

func (t *hyperKeyTracker) HandleEvent(event keyboard.RawKeyEvent) (keyboard.Key, bool) {
	if event.Key == keyboard.KeyCapsLock {
		if t.passthroughCapsEvents > 0 {
			t.passthroughCapsEvents--
			return keyboard.KeyUnknown, false
		}

		if event.Type == keyboard.EventTypeKeyDown {
			t.capsPressed = true
			t.comboTriggered = false
			return keyboard.KeyUnknown, true
		}

		shouldReplayCaps := !t.comboTriggered
		t.capsPressed = false
		t.comboTriggered = false
		t.pressedKeys = map[keyboard.Key]bool{}
		if shouldReplayCaps {
			t.passthroughCapsEvents = 2
			util.Go(util.NewTraceContext(), "replay single Caps Lock tap", func() {
				if err := keyboard.SimulateCapsLockTap(); err != nil {
					util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("failed to replay single Caps Lock tap: %s", err.Error()))
				}
			})
		}
		return keyboard.KeyUnknown, true
	}

	if !t.capsPressed {
		return keyboard.KeyUnknown, false
	}

	if event.Type == keyboard.EventTypeKeyUp {
		delete(t.pressedKeys, event.Key)
		return keyboard.KeyUnknown, true
	}

	t.comboTriggered = true
	if event.Key == keyboard.KeyUnknown || t.pressedKeys[event.Key] {
		return keyboard.KeyUnknown, true
	}

	t.pressedKeys[event.Key] = true
	return event.Key, true
}

var (
	hyperKeyMu        sync.Mutex
	hyperKeyCallbacks = map[keyboard.Key]func(){}
	hyperKeyListener  keyboard.RawKeySubscription
	hyperKeyState     = newHyperKeyTracker()
)

func registerHyperHotKey(key keyboard.Key, callback func()) error {
	hyperKeyMu.Lock()
	if _, exists := hyperKeyCallbacks[key]; exists {
		hyperKeyMu.Unlock()
		return fmt.Errorf("hyper hotkey already registered for key: %s", key.Character())
	}
	hyperKeyCallbacks[key] = callback
	hyperKeyMu.Unlock()

	if hyperKeyListener != nil {
		return nil
	}

	listener, err := keyboard.AddRawKeyListener(func(event keyboard.RawKeyEvent) bool {
		triggeredKey, consume := handleHyperKeyEvent(event)
		if triggeredKey == keyboard.KeyUnknown {
			return consume
		}

		hyperKeyMu.Lock()
		callback := hyperKeyCallbacks[triggeredKey]
		hyperKeyMu.Unlock()
		if callback != nil {
			util.Go(util.NewTraceContext(), "hyper hotkey callback", callback)
		}

		return consume
	})
	if err != nil {
		hyperKeyMu.Lock()
		delete(hyperKeyCallbacks, key)
		hyperKeyMu.Unlock()
		return err
	}

	hyperKeyListener = listener
	return nil
}

func unregisterHyperHotKey(key keyboard.Key) {
	hyperKeyMu.Lock()
	delete(hyperKeyCallbacks, key)
	shouldClose := len(hyperKeyCallbacks) == 0
	hyperKeyMu.Unlock()

	if !shouldClose {
		return
	}

	if hyperKeyListener != nil {
		_ = hyperKeyListener.Close()
		hyperKeyListener = nil
	}

	hyperKeyMu.Lock()
	hyperKeyState = newHyperKeyTracker()
	hyperKeyMu.Unlock()
}

func handleHyperKeyEvent(event keyboard.RawKeyEvent) (keyboard.Key, bool) {
	hyperKeyMu.Lock()
	defer hyperKeyMu.Unlock()
	return hyperKeyState.HandleEvent(event)
}
