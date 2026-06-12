package hotkey

import (
	"fmt"
	"runtime"
	"sync"
	"wox/util"
	"wox/util/keyboard"
)

const (
	darwinSyntheticCapsEventIgnoreMs = 150
	darwinHyperComboStartWindowMs    = 500
)

type hyperKeyTracker struct {
	capsPressed           bool
	comboTriggered        bool
	capsPressedAt         int64
	capsLockStateCaptured bool
	capsLockStateBefore   bool
	capsLockStateRestored bool
	passthroughCapsEvents int
	ignoreCapsEventsUntil int64
	pressedKeys           map[keyboard.Key]bool
}

func newHyperKeyTracker() *hyperKeyTracker {
	return &hyperKeyTracker{
		pressedKeys: map[keyboard.Key]bool{},
	}
}

func (t *hyperKeyTracker) HandleEvent(event keyboard.RawKeyEvent, allowCapsLockReplay bool) (keyboard.Key, bool) {
	if event.Key == keyboard.KeyCapsLock {
		if runtime.GOOS == "darwin" {
			return t.handleDarwinCapsLockEvent(event, allowCapsLockReplay)
		}

		return t.handleDefaultCapsLockEvent(event, allowCapsLockReplay)
	}

	if runtime.GOOS == "darwin" {
		return t.handleDarwinNonCapsLockEvent(event, allowCapsLockReplay)
	}

	return t.handleDefaultNonCapsLockEvent(event)
}

func (t *hyperKeyTracker) handleDefaultCapsLockEvent(event keyboard.RawKeyEvent, allowCapsLockReplay bool) (keyboard.Key, bool) {
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

		shouldReplayCaps := allowCapsLockReplay && shouldReplayCapsLockTap(t.comboTriggered)
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

	return keyboard.KeyUnknown, false
}

func (t *hyperKeyTracker) handleDefaultNonCapsLockEvent(event keyboard.RawKeyEvent) (keyboard.Key, bool) {
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

func (t *hyperKeyTracker) handleDarwinCapsLockEvent(event keyboard.RawKeyEvent, allowCapsLockStateUpdate bool) (keyboard.Key, bool) {
	now := util.GetSystemTimestamp()
	if t.ignoreCapsEventsUntil > 0 && now <= t.ignoreCapsEventsUntil {
		return keyboard.KeyUnknown, false
	}

	if !t.capsPressed {
		t.capsPressed = true
		t.comboTriggered = false
		t.capsPressedAt = now
		t.capsLockStateCaptured = true
		t.capsLockStateBefore = event.Type != keyboard.EventTypeKeyDown
		t.capsLockStateRestored = false
		t.pressedKeys = map[keyboard.Key]bool{}
		return keyboard.KeyUnknown, true
	}

	if t.comboTriggered {
		t.finishDarwinHyperSequence(allowCapsLockStateUpdate, "caps-state-transition")
		return keyboard.KeyUnknown, true
	}

	t.resetCapsSequence()
	return keyboard.KeyUnknown, true
}

// handleDarwinNonCapsLockEvent treats Caps Lock as a Hyper prefix even though macOS reports
// Caps Lock as lock-state transitions instead of a normal physical down/up pair.
func (t *hyperKeyTracker) handleDarwinNonCapsLockEvent(event keyboard.RawKeyEvent, allowCapsLockStateUpdate bool) (keyboard.Key, bool) {
	if !t.capsPressed {
		return keyboard.KeyUnknown, false
	}

	if t.comboTriggered && len(t.pressedKeys) == 0 && !keyboard.IsKeyPressed(keyboard.KeyCapsLock) {
		t.finishDarwinHyperSequence(allowCapsLockStateUpdate, "caps-released-before-next-key")
		return keyboard.KeyUnknown, false
	}

	if event.Type == keyboard.EventTypeKeyUp {
		if !t.comboTriggered {
			return keyboard.KeyUnknown, false
		}

		delete(t.pressedKeys, event.Key)
		if len(t.pressedKeys) == 0 {
			t.finishDarwinHyperSequence(allowCapsLockStateUpdate, "combo-keys-released")
		}
		return keyboard.KeyUnknown, true
	}

	if !t.comboTriggered && !t.shouldTreatDarwinKeyAsCombo() {
		t.resetCapsSequence()
		return keyboard.KeyUnknown, false
	}

	t.comboTriggered = true
	t.restoreDarwinCapsLockState(allowCapsLockStateUpdate, "combo-triggered")
	if event.Key == keyboard.KeyUnknown || t.pressedKeys[event.Key] {
		return keyboard.KeyUnknown, true
	}

	t.pressedKeys[event.Key] = true
	return event.Key, true
}

// shouldTreatDarwinKeyAsCombo avoids turning ordinary typing into Hyper after a standalone Caps tap.
func (t *hyperKeyTracker) shouldTreatDarwinKeyAsCombo() bool {
	if t.capsPressedAt == 0 {
		return false
	}
	return util.GetSystemTimestamp()-t.capsPressedAt <= darwinHyperComboStartWindowMs
}

// finishDarwinHyperSequence clears the synthetic Hyper state once the combo is no longer active.
func (t *hyperKeyTracker) finishDarwinHyperSequence(allowCapsLockStateUpdate bool, reason string) {
	if t.comboTriggered {
		t.restoreDarwinCapsLockState(allowCapsLockStateUpdate, reason)
	}
	t.resetCapsSequence()
}

// restoreDarwinCapsLockState undoes the native Caps toggle caused by using Caps as Hyper.
func (t *hyperKeyTracker) restoreDarwinCapsLockState(allowCapsLockStateUpdate bool, reason string) {
	if !allowCapsLockStateUpdate || !t.capsLockStateCaptured || t.capsLockStateRestored {
		return
	}

	targetState := t.capsLockStateBefore
	t.capsLockStateRestored = true
	t.ignoreCapsEventsUntil = util.GetSystemTimestamp() + darwinSyntheticCapsEventIgnoreMs
	util.Go(util.NewTraceContext(), "restore Caps Lock state after Hyper combo", func() {
		if err := keyboard.SetCapsLockState(targetState); err != nil {
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("failed to restore Caps Lock state after Hyper combo: targetState=%t reason=%s err=%s", targetState, reason, err.Error()))
		}
	})
}

func (t *hyperKeyTracker) resetCapsSequence() {
	t.capsPressed = false
	t.comboTriggered = false
	t.capsPressedAt = 0
	t.capsLockStateCaptured = false
	t.capsLockStateBefore = false
	t.capsLockStateRestored = false
	t.pressedKeys = map[keyboard.Key]bool{}
}

// shouldReplayCapsLockTap keeps Caps Lock's native toggle behavior aligned with the Hyper state machine.
func shouldReplayCapsLockTap(comboTriggered bool) bool {
	return !comboTriggered
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
	var listenerToClose keyboard.RawKeySubscription

	hyperKeyMu.Lock()
	hyperKeyRecorder = recorder
	hyperKeyState = newHyperKeyTracker()
	if recorder == nil && len(hyperKeyCallbacks) == 0 && hyperKeyListener != nil {
		listenerToClose = hyperKeyListener
		hyperKeyListener = nil
	}
	hyperKeyMu.Unlock()

	if listenerToClose != nil {
		if err := listenerToClose.Close(); err != nil {
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("failed to close Hyper key listener after recorder stopped: %s", err.Error()))
		}
	}
	if recorder != nil {
		if err := ensureHyperKeyListener(); err != nil {
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("failed to start Hyper key recorder listener: %s", err.Error()))
		}
	}
}

func registerHyperHotKey(key keyboard.Key, callback func()) error {
	hyperKeyMu.Lock()
	if _, exists := hyperKeyCallbacks[key]; exists {
		hyperKeyMu.Unlock()
		return fmt.Errorf("hyper hotkey already registered for key: %s", key.Character())
	}
	hyperKeyCallbacks[key] = callback
	hyperKeyMu.Unlock()

	if err := ensureHyperKeyListener(); err != nil {
		hyperKeyMu.Lock()
		delete(hyperKeyCallbacks, key)
		hyperKeyMu.Unlock()
		return err
	}

	return nil
}

// ensureHyperKeyListener starts the shared raw listener used by both Hyper recording and registered Hyper hotkeys.
func ensureHyperKeyListener() error {
	hyperKeyMu.Lock()
	if hyperKeyListener != nil {
		hyperKeyMu.Unlock()
		return nil
	}
	hyperKeyMu.Unlock()

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
		return err
	}

	hyperKeyMu.Lock()
	if hyperKeyListener != nil {
		hyperKeyMu.Unlock()
		_ = listener.Close()
		return nil
	}
	hyperKeyListener = listener
	hyperKeyMu.Unlock()
	return nil
}

func unregisterHyperHotKey(key keyboard.Key) {
	var listenerToClose keyboard.RawKeySubscription

	hyperKeyMu.Lock()
	delete(hyperKeyCallbacks, key)
	shouldClose := len(hyperKeyCallbacks) == 0 && hyperKeyRecorder == nil
	if shouldClose && hyperKeyListener != nil {
		listenerToClose = hyperKeyListener
		hyperKeyListener = nil
	}
	hyperKeyMu.Unlock()

	if !shouldClose {
		return
	}

	if listenerToClose != nil {
		if err := listenerToClose.Close(); err != nil {
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("failed to close Hyper key listener after unregister: %s", err.Error()))
		}
	}

	hyperKeyMu.Lock()
	hyperKeyState = newHyperKeyTracker()
	hyperKeyMu.Unlock()
}

func handleHyperKeyEvent(event keyboard.RawKeyEvent) (keyboard.Key, bool, func(string)) {
	hyperKeyMu.Lock()
	defer hyperKeyMu.Unlock()
	recorder := hyperKeyRecorder
	triggeredKey, consume := hyperKeyState.HandleEvent(event, recorder == nil || runtime.GOOS == "darwin")
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
