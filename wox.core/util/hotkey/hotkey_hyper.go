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

func (t *hyperKeyTracker) HandleEvent(event keyboard.RawKeyEvent, replaySingleCapsTap bool) (keyboard.Key, bool) {
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

		shouldReplayCaps := replaySingleCapsTap && !t.comboTriggered
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
	hyperKeyRecorder  func(string)
)

// SetHyperKeyRecorder forwards Hyper key combinations to the active UI recorder.
func SetHyperKeyRecorder(recorder func(string)) {
	hyperKeyMu.Lock()
	hyperKeyRecorder = recorder
	hyperKeyState = newHyperKeyTracker()
	hyperKeyMu.Unlock()
}

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
		triggeredKey, consume, recorder := handleHyperKeyEvent(event)
		if triggeredKey == keyboard.KeyUnknown {
			return consume
		}

		if recorder != nil {
			if hotkeyStr := hyperKeyToHotkeyString(triggeredKey); hotkeyStr != "" {
				util.Go(util.NewTraceContext(), "record hyper hotkey in UI", func() {
					recorder(hotkeyStr)
				})
			}
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

func handleHyperKeyEvent(event keyboard.RawKeyEvent) (keyboard.Key, bool, func(string)) {
	hyperKeyMu.Lock()
	defer hyperKeyMu.Unlock()
	recorder := hyperKeyRecorder
	triggeredKey, consume := hyperKeyState.HandleEvent(event, recorder == nil)
	return triggeredKey, consume, recorder
}

func hyperKeyToHotkeyString(key keyboard.Key) string {
	if character := key.Character(); character != "" {
		return "hyper+" + character
	}

	switch key {
	case keyboard.KeySpace:
		return "hyper+space"
	case keyboard.KeyReturn:
		return "hyper+enter"
	case keyboard.KeyEscape:
		return "hyper+escape"
	case keyboard.KeyTab:
		return "hyper+tab"
	case keyboard.KeyDelete:
		return "hyper+delete"
	case keyboard.KeyLeft:
		return "hyper+left"
	case keyboard.KeyRight:
		return "hyper+right"
	case keyboard.KeyUp:
		return "hyper+up"
	case keyboard.KeyDown:
		return "hyper+down"
	case keyboard.KeyF1:
		return "hyper+f1"
	case keyboard.KeyF2:
		return "hyper+f2"
	case keyboard.KeyF3:
		return "hyper+f3"
	case keyboard.KeyF4:
		return "hyper+f4"
	case keyboard.KeyF5:
		return "hyper+f5"
	case keyboard.KeyF6:
		return "hyper+f6"
	case keyboard.KeyF7:
		return "hyper+f7"
	case keyboard.KeyF8:
		return "hyper+f8"
	case keyboard.KeyF9:
		return "hyper+f9"
	case keyboard.KeyF10:
		return "hyper+f10"
	case keyboard.KeyF11:
		return "hyper+f11"
	case keyboard.KeyF12:
		return "hyper+f12"
	default:
		return ""
	}
}
