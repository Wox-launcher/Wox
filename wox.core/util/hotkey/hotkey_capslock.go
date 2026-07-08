package hotkey

import (
	"fmt"
	"runtime"
	"sync"
	"time"
	"wox/util"
	"wox/util/keyboard"
)

const (
	darwinSyntheticCapsEventIgnoreMs        = 150
	capsLockComboCallbackReleaseMaxWait     = 1500 * time.Millisecond
	capsLockComboCallbackReleasePollDelay   = 15 * time.Millisecond
	capsLockComboCallbackReleaseSettleDelay = 15 * time.Millisecond
)

type capsLockComboTracker struct {
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

func newCapsLockComboTracker() *capsLockComboTracker {
	return &capsLockComboTracker{
		pressedKeys: map[keyboard.Key]bool{},
	}
}

func (t *capsLockComboTracker) HandleEvent(event keyboard.RawKeyEvent, allowCapsLockReplay bool) (keyboard.Key, bool) {
	if event.Key == keyboard.KeyCapsLock {
		if runtime.GOOS == "darwin" {
			return t.handleDarwinCapsLockEvent(event, allowCapsLockReplay)
		}
		// Both Windows and Linux use the same capture-and-restore approach:
		// capture the CapsLock state on key-down, then explicitly set the
		// target state on key-up. On Linux, the system also sees the raw
		// CapsLock events (evdev is read-only), so the restore step toggles
		// the state back if a combo was triggered.
		return t.handleStateCaptureCapsLockEvent(event, allowCapsLockReplay)
	}

	if runtime.GOOS == "darwin" {
		return t.handleDarwinNonCapsLockEvent(event, allowCapsLockReplay)
	}

	return t.handleDefaultNonCapsLockEvent(event)
}

func (t *capsLockComboTracker) handleDefaultCapsLockEvent(event keyboard.RawKeyEvent, allowCapsLockReplay bool) (keyboard.Key, bool) {
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

		shouldReplayCaps := allowCapsLockReplay && shouldReplayCapsLockPress(t.comboTriggered)
		t.capsPressed = false
		t.comboTriggered = false
		t.pressedKeys = map[keyboard.Key]bool{}
		if shouldReplayCaps {
			t.passthroughCapsEvents = 2
			util.Go(util.NewTraceContext(), "replay single Caps Lock press", func() {
				if err := keyboard.SimulateCapsLockPress(); err != nil {
					util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("failed to replay single Caps Lock press: %s", err.Error()))
				}
			})
		}
		return keyboard.KeyUnknown, true
	}

	return keyboard.KeyUnknown, false
}

// handleStateCaptureCapsLockEvent keeps Caps Lock behavior aligned by setting
// the final toggle state explicitly after the combo sequence completes.
//
// On Windows: the WH_KEYBOARD_LL hook consumes the raw CapsLock event, so the
// system never toggles. We capture the pre-toggle state on key-down and set
// the target state explicitly on key-up.
//
// On Linux (evdev): we can't consume the raw event, so the kernel toggles
// CapsLock before our handler sees the key-down. We capture the post-toggle
// state on key-down, then on key-up we toggle back if a combo was triggered
// (undoing the system's toggle) or leave it as-is if CapsLock was pressed
// alone (preserving the normal toggle behavior).
func (t *capsLockComboTracker) handleStateCaptureCapsLockEvent(event keyboard.RawKeyEvent, allowCapsLockStateUpdate bool) (keyboard.Key, bool) {
	if t.passthroughCapsEvents > 0 {
		// These events are from our own simulated CapsLock press; let them
		// pass through to the system without processing.
		t.passthroughCapsEvents--
		return keyboard.KeyUnknown, false
	}

	if event.Type == keyboard.EventTypeKeyDown {
		t.capsPressed = true
		t.comboTriggered = false
		t.capsLockStateCaptured = true
		t.capsLockStateBefore = keyboard.IsCapsLockEnabled()
		t.capsLockStateRestored = false
		t.pressedKeys = map[keyboard.Key]bool{}
		return keyboard.KeyUnknown, true
	}

	allowSetState := allowCapsLockStateUpdate && t.capsLockStateCaptured

	if runtime.GOOS == "linux" {
		// On Linux, the system already toggled CapsLock on key-down.
		// If a combo was triggered, toggle back to undo the system's toggle.
		// If CapsLock was pressed alone, leave the system's toggle as-is.
		shouldUndoToggle := t.comboTriggered
		t.resetCapsSequence()
		if allowSetState && shouldUndoToggle {
			// On Linux, the CapsLock undo press is injected via /dev/uinput,
			// which creates a separate virtual keyboard device. Our evdev
			// listener only reads physical keyboard devices (/dev/input/event*),
			// so it never sees the injected events. This means we do NOT need
			// the passthroughCapsEvents mechanism (unlike Windows where the
			// WH_KEYBOARD_LL hook sees injected events from the same device).
			currentState := keyboard.IsCapsLockEnabled()
			setCapsLockStateAsync(!currentState, "linux-undo-caps-toggle")
		}
		return keyboard.KeyUnknown, true
	}

	// Windows: the system never toggled (event was consumed). Set the target
	// state explicitly.
	targetState := t.capsLockStateBefore
	if !t.comboTriggered {
		targetState = !targetState
	}
	t.resetCapsSequence()
	if allowSetState {
		setCapsLockStateAsync(targetState, "windows-caps-lock-sequence")
	}
	return keyboard.KeyUnknown, true
}

func (t *capsLockComboTracker) handleDefaultNonCapsLockEvent(event keyboard.RawKeyEvent) (keyboard.Key, bool) {
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

func (t *capsLockComboTracker) handleDarwinCapsLockEvent(event keyboard.RawKeyEvent, allowCapsLockStateUpdate bool) (keyboard.Key, bool) {
	now := util.GetSystemTimestamp()
	if t.ignoreCapsEventsUntil > 0 && now <= t.ignoreCapsEventsUntil {
		return keyboard.KeyUnknown, false
	}

	// When the IOHID physical keyboard monitor is available, use it to
	// determine the physical Caps Lock key state (the gold standard).
	// When it is NOT available (some keyboards/macOS versions don't deliver
	// Caps Lock HID events), fall back to the kCGEventFlagsChanged toggle
	// event itself: EventTypeKeyDown means Caps Lock was toggled ON (key
	// pressed), EventTypeKeyUp means it was toggled OFF (key pressed again).
	iohidAvailable := event.NativeCapsLockStateAvailable

	if !t.capsPressed {
		// Start combo mode.
		if iohidAvailable {
			// IOHID path: trust the physical key state.
			if !event.NativeCapsLockPressed {
				return keyboard.KeyUnknown, false
			}
		} else {
			// Fallback path: only start on a toggle-ON event (KeyDown).
			// A KeyUp event here means Caps Lock is being toggled OFF and
			// we were not in combo mode, so just let it pass through.
			if event.Type != keyboard.EventTypeKeyDown {
				return keyboard.KeyUnknown, false
			}
		}

		t.capsPressed = true
		t.comboTriggered = false
		t.capsPressedAt = now
		t.capsLockStateCaptured = true
		t.capsLockStateBefore = event.Type != keyboard.EventTypeKeyDown
		t.capsLockStateRestored = false
		t.pressedKeys = map[keyboard.Key]bool{}
		return keyboard.KeyUnknown, true
	}

	// Caps Lock is already pressed (we're in combo mode). Another
	// kCGEventFlagsChanged means the user pressed Caps Lock again to
	// toggle it OFF. End the current combo sequence.
	if t.comboTriggered {
		t.finishDarwinCapsLockComboSequence(allowCapsLockStateUpdate, "caps-state-transition")
		return keyboard.KeyUnknown, true
	}

	// Caps Lock was pressed but no combo key was pressed – the user
	// toggled Caps Lock off without using it as a modifier. Reset the
	// combo state and consume the event. The system has already toggled
	// the state back, which matches capsLockStateBefore, so no explicit
	// restoration is needed. Consuming prevents other apps from seeing
	// a toggle-OFF event without a corresponding toggle-ON.
	t.resetCapsSequence()
	return keyboard.KeyUnknown, true
}

// handleDarwinNonCapsLockEvent treats Caps Lock as a combo prefix even though macOS reports
// Caps Lock as lock-state transitions instead of a normal physical down/up pair.
func (t *capsLockComboTracker) handleDarwinNonCapsLockEvent(event keyboard.RawKeyEvent, allowCapsLockStateUpdate bool) (keyboard.Key, bool) {
	if !t.capsPressed {
		// Recovery path: a non-CapsLock key event arrived while we're not in
		// combo mode. This can happen when a Caps Lock state transition
		// reset the Go state while IOHID still reports the physical key as
		// held. Only attempt recovery when IOHID is available.
		if event.Type != keyboard.EventTypeKeyDown || !event.NativeCapsLockStateAvailable || !event.NativeCapsLockPressed {
			return keyboard.KeyUnknown, false
		}

		t.capsPressed = true
		t.comboTriggered = false
		t.capsPressedAt = util.GetSystemTimestamp()
		t.capsLockStateCaptured = false
		t.capsLockStateBefore = false
		t.capsLockStateRestored = false
		t.pressedKeys = map[keyboard.Key]bool{}
	}

	if t.comboTriggered && len(t.pressedKeys) == 0 && !t.isDarwinCapsLockStillPressed(event) {
		t.finishDarwinCapsLockComboSequence(allowCapsLockStateUpdate, "caps-released-before-next-key")
		return keyboard.KeyUnknown, false
	}

	if event.Type == keyboard.EventTypeKeyUp {
		if !t.comboTriggered {
			return keyboard.KeyUnknown, false
		}

		delete(t.pressedKeys, event.Key)
		if len(t.pressedKeys) == 0 {
			t.finishDarwinCapsLockComboSequence(allowCapsLockStateUpdate, "combo-keys-released")
		}
		return keyboard.KeyUnknown, true
	}

	if !t.comboTriggered && !t.shouldTreatDarwinKeyAsCombo(event) {
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

// shouldTreatDarwinKeyAsCombo decides whether a non-CapsLock key event should
// be treated as part of a Caps Lock combo. When the IOHID physical keyboard
// monitor is available, it trusts the physical Caps Lock state. When IOHID is
// not available, it trusts the Go-level capsPressed state set by
// handleDarwinCapsLockEvent (the kCGEventFlagsChanged fallback path).
func (t *capsLockComboTracker) shouldTreatDarwinKeyAsCombo(event keyboard.RawKeyEvent) bool {
	if event.NativeCapsLockStateAvailable {
		return event.NativeCapsLockPressed
	}
	// Fallback: if we're in combo mode (capsPressed was set by the
	// flagsChanged event), treat the key as part of the combo.
	return true
}

// isDarwinCapsLockStillPressed reports whether the physical Caps Lock key is
// still held down. When IOHID is available, it trusts the physical state. When
// IOHID is not available, it assumes the key is still pressed so that the combo
// sequence ends via the normal "combo-keys-released" path instead of being
// cut short prematurely.
func (t *capsLockComboTracker) isDarwinCapsLockStillPressed(event keyboard.RawKeyEvent) bool {
	if event.NativeCapsLockStateAvailable {
		return event.NativeCapsLockPressed
	}
	// Fallback: assume still pressed; the combo will end when all combo
	// keys are released (handled by the "combo-keys-released" path).
	return true
}

// finishDarwinCapsLockComboSequence clears the synthetic Caps Lock combo state once the combo is no longer active.
func (t *capsLockComboTracker) finishDarwinCapsLockComboSequence(allowCapsLockStateUpdate bool, reason string) {
	if t.comboTriggered {
		t.restoreDarwinCapsLockState(allowCapsLockStateUpdate, reason)
	}
	t.resetCapsSequence()
}

// restoreDarwinCapsLockState undoes the native Caps toggle caused by using Caps Lock as a combo prefix.
func (t *capsLockComboTracker) restoreDarwinCapsLockState(allowCapsLockStateUpdate bool, reason string) {
	if !allowCapsLockStateUpdate || !t.capsLockStateCaptured || t.capsLockStateRestored {
		return
	}

	targetState := t.capsLockStateBefore
	t.capsLockStateRestored = true
	t.ignoreCapsEventsUntil = util.GetSystemTimestamp() + darwinSyntheticCapsEventIgnoreMs
	setCapsLockStateAsync(targetState, reason)
}

func setCapsLockStateAsync(targetState bool, reason string) {
	util.Go(util.NewTraceContext(), "set Caps Lock state after Caps Lock combo", func() {
		if err := keyboard.SetCapsLockState(targetState); err != nil {
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("failed to set Caps Lock state: targetState=%t reason=%s err=%s", targetState, reason, err.Error()))
		}
	})
}

func (t *capsLockComboTracker) resetCapsSequence() {
	t.capsPressed = false
	t.comboTriggered = false
	t.capsPressedAt = 0
	t.capsLockStateCaptured = false
	t.capsLockStateBefore = false
	t.capsLockStateRestored = false
	t.pressedKeys = map[keyboard.Key]bool{}
}

// shouldReplayCapsLockPress keeps Caps Lock's native toggle behavior aligned with the combo state machine.
func shouldReplayCapsLockPress(comboTriggered bool) bool {
	return !comboTriggered
}

var (
	capsLockComboMu        sync.Mutex
	capsLockComboCallbacks = map[keyboard.Key]func(){}
	capsLockComboListener  keyboard.RawKeySubscription
	capsLockComboState     = newCapsLockComboTracker()
)

func registerCapsLockComboHotKey(key keyboard.Key, callback func()) error {
	capsLockComboMu.Lock()
	if _, exists := capsLockComboCallbacks[key]; exists {
		capsLockComboMu.Unlock()
		return fmt.Errorf("caps lock hotkey already registered for key: %s", key.Character())
	}
	capsLockComboCallbacks[key] = callback
	capsLockComboMu.Unlock()

	if err := ensureCapsLockComboListener(); err != nil {
		capsLockComboMu.Lock()
		delete(capsLockComboCallbacks, key)
		capsLockComboMu.Unlock()
		return err
	}

	return nil
}

// ensureCapsLockComboListener starts the shared raw listener used by registered Caps Lock hotkeys.
func ensureCapsLockComboListener() error {
	capsLockComboMu.Lock()
	if capsLockComboListener != nil {
		capsLockComboMu.Unlock()
		return nil
	}
	capsLockComboMu.Unlock()

	listener, err := addRawKeyListener(func(event keyboard.RawKeyEvent) bool {
		triggeredKey, consume := handleCapsLockComboEvent(event)
		if triggeredKey == keyboard.KeyUnknown {
			return consume
		}

		capsLockComboMu.Lock()
		callback := capsLockComboCallbacks[triggeredKey]
		capsLockComboMu.Unlock()
		if callback != nil {
			util.Go(util.NewTraceContext(), "caps lock hotkey callback", func() {
				waitForCapsLockComboRelease(triggeredKey)
				// On Linux/Wayland, the system sees the combo key (e.g. 'A')
				// because evdev is read-only, so it types a stray character
				// into the focused input field. Inject a Backspace via uinput
				// to delete it before showing the Wox UI.
				if runtime.GOOS == "linux" {
					if err := keyboard.SimulateBackspace(); err != nil {
						util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf(
							"failed to delete stray combo character: %s", err.Error()))
					}
				}
				callback()
			})
		}

		return consume
	})
	if err != nil {
		return err
	}

	capsLockComboMu.Lock()
	if capsLockComboListener != nil {
		capsLockComboMu.Unlock()
		_ = listener.Close()
		return nil
	}
	capsLockComboListener = listener
	capsLockComboMu.Unlock()
	return nil
}

func unregisterCapsLockComboHotKey(key keyboard.Key) {
	var listenerToClose keyboard.RawKeySubscription

	capsLockComboMu.Lock()
	delete(capsLockComboCallbacks, key)
	shouldClose := len(capsLockComboCallbacks) == 0
	if shouldClose && capsLockComboListener != nil {
		listenerToClose = capsLockComboListener
		capsLockComboListener = nil
	}
	capsLockComboMu.Unlock()

	if !shouldClose {
		return
	}

	if listenerToClose != nil {
		if err := listenerToClose.Close(); err != nil {
			util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("failed to close Caps Lock combo listener after unregister: %s", err.Error()))
		}
	}

	capsLockComboMu.Lock()
	capsLockComboState = newCapsLockComboTracker()
	capsLockComboMu.Unlock()
}

func handleCapsLockComboEvent(event keyboard.RawKeyEvent) (keyboard.Key, bool) {
	capsLockComboMu.Lock()
	defer capsLockComboMu.Unlock()
	return capsLockComboState.HandleEvent(event, true)
}

// waitForCapsLockComboRelease keeps synthetic keyboard input from being swallowed by the active Caps Lock raw-key sequence.
func waitForCapsLockComboRelease(triggeredKey keyboard.Key) {
	deadline := time.Now().Add(capsLockComboCallbackReleaseMaxWait)
	for time.Now().Before(deadline) {
		if !keyboard.IsKeyPressed(keyboard.KeyCapsLock) && (triggeredKey == keyboard.KeyUnknown || !keyboard.IsKeyPressed(triggeredKey)) {
			time.Sleep(capsLockComboCallbackReleaseSettleDelay)
			return
		}

		time.Sleep(capsLockComboCallbackReleasePollDelay)
	}
}

func capsLockComboToHotkeyString(key keyboard.Key) string {
	if character := key.Character(); character != "" {
		return "capslock+" + character
	}

	switch key {
	case keyboard.KeySpace:
		return "capslock+space"
	case keyboard.KeyReturn:
		return "capslock+enter"
	case keyboard.KeyEscape:
		return "capslock+escape"
	case keyboard.KeyTab:
		return "capslock+tab"
	case keyboard.KeyDelete:
		return "capslock+delete"
	case keyboard.KeyLeft:
		return "capslock+left"
	case keyboard.KeyRight:
		return "capslock+right"
	case keyboard.KeyUp:
		return "capslock+up"
	case keyboard.KeyDown:
		return "capslock+down"
	case keyboard.KeyF1:
		return "capslock+f1"
	case keyboard.KeyF2:
		return "capslock+f2"
	case keyboard.KeyF3:
		return "capslock+f3"
	case keyboard.KeyF4:
		return "capslock+f4"
	case keyboard.KeyF5:
		return "capslock+f5"
	case keyboard.KeyF6:
		return "capslock+f6"
	case keyboard.KeyF7:
		return "capslock+f7"
	case keyboard.KeyF8:
		return "capslock+f8"
	case keyboard.KeyF9:
		return "capslock+f9"
	case keyboard.KeyF10:
		return "capslock+f10"
	case keyboard.KeyF11:
		return "capslock+f11"
	case keyboard.KeyF12:
		return "capslock+f12"
	default:
		return ""
	}
}
