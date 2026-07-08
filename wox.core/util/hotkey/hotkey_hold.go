package hotkey

import (
	"fmt"
	"sync"
	"time"
	"wox/util"
	"wox/util/keyboard"
)

// holdCallback stores the release callback for a key that is being tracked
// for release events only (used by normal/double-modifier hotkeys in hold
// mode). The callback persists across multiple press/release cycles.
type holdCallback struct {
	onRelease func()
	key       keyboard.Key
}

// holdModifierCallback stores both press and release callbacks for a
// hold-modifier hotkey. This mode does not register a system hotkey; the raw
// key listener handles both key down (press) and key up (release).
//
// State machine (all fields guarded by holdTrackerMu):
//   - pressTimer != nil, pressFired == false: key is held but the minimum hold
//     duration has not yet elapsed. A quick tap or any other key press while in
//     this state cancels the timer and suppresses both callbacks.
//   - pressTimer == nil, pressFired == true: the key was held long enough and
//     onPress has fired. The next key-up for this key triggers onRelease. Any
//     other key-down first ends the hold immediately and suppresses the later
//     key-up release callback.
//
// This ensures the action only fires when the modifier key is held *alone* for
// the full hold duration. Pressing another key (e.g. space while holding cmd
// for cmd+space) cancels the action.
type holdModifierCallback struct {
	onPress    func()
	onRelease  func()
	key        keyboard.Key
	pressTimer *time.Timer
	pressFired bool
}

type holdModifierRelease struct {
	key      keyboard.Key
	callback func()
}

// holdModifierPressDelay is the minimum time a hold-modifier key must remain
// pressed alone before onPress fires. Taps shorter than this, or holds
// interrupted by another key press, are treated as accidental and ignored,
// matching the "press and hold" semantics users expect from hold mode.
const holdModifierPressDelay = 200 * time.Millisecond

var (
	holdCallbacks         = util.NewHashMap[keyboard.Key, *holdCallback]()
	holdModifierCallbacks = util.NewHashMap[keyboard.Key, *holdModifierCallback]()
	holdKeyListener       keyboard.RawKeySubscription
	holdTrackerMu         sync.Mutex
)

// ensureHoldKeyListener creates the shared raw key listener if it is not
// already active. The listener dispatches events to both holdCallbacks
// (release-only) and holdModifierCallbacks (press + release).
func ensureHoldKeyListener() error {
	if holdKeyListener != nil {
		return nil
	}

	listener, err := keyboard.AddRawKeyListener(func(event keyboard.RawKeyEvent) bool {
		holdTrackerMu.Lock()

		if event.Type == keyboard.EventTypeKeyDown {
			releases, canceledOther := cancelHoldModifierPressesForChord(event.Key)
			if canceledOther {
				holdTrackerMu.Unlock()
				dispatchHoldModifierReleases(releases)
				return false
			}

			// If the hold-modifier key itself goes down (initial press or OS
			// key-repeat), (re)arm its minimum-hold timer.
			if mcb, ok := holdModifierCallbacks.Load(event.Key); ok && mcb != nil {
				if isOtherSpecificModifierPressed(event.Key) {
					releases, _ = cancelHoldModifierPressesForChord(keyboard.KeyUnknown)
					holdTrackerMu.Unlock()
					dispatchHoldModifierReleases(releases)
					return false
				}
				util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf("hold-modifier keyDown: key=%s timer=%v fired=%v", event.Key.Character(), mcb.pressTimer != nil, mcb.pressFired))
				armHoldModifierPress(mcb, event.Key)
				holdTrackerMu.Unlock()
				return false
			}

			// A *different* key going down means the user is forming a chord
			// (e.g. cmd+space), not holding the modifier alone. End every
			// active hold-modifier press so the action does not keep running.
			releases, _ = cancelHoldModifierPressesForChord(keyboard.KeyUnknown)
			holdTrackerMu.Unlock()
			dispatchHoldModifierReleases(releases)
			return false
		}

		if event.Type == keyboard.EventTypeKeyUp {
			// Check hold-modifier callbacks first (press + release mode).
			mcb, mok := holdModifierCallbacks.Load(event.Key)
			if mok && mcb != nil {
				timer := mcb.pressTimer
				fired := mcb.pressFired
				mcb.pressTimer = nil
				mcb.pressFired = false
				holdTrackerMu.Unlock()

				util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf("hold-modifier keyUp: key=%s timer=%v fired=%v", event.Key.Character(), timer != nil, fired))

				if timer != nil {
					// Timer was still pending: the key was released (or a chord
					// already cancelled it) before the hold delay elapsed.
					// Suppress both callbacks — this was a tap, not a hold.
					timer.Stop()
					return false
				}

				// Timer already fired: onPress ran, so dispatch onRelease.
				if fired && mcb.onRelease != nil {
					util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf("hold-modifier dispatching onRelease: key=%s", event.Key.Character()))
					util.Go(util.NewTraceContext(), fmt.Sprintf("hold-modifier hotkey release: %s", event.Key.Character()), func() {
						mcb.onRelease()
					})
				} else {
					util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf("hold-modifier keyUp NOT dispatching: fired=%v onRelease=%v", fired, mcb.onRelease != nil))
				}
				return false
			}

			// Fall back to release-only hold callbacks.
			hcb, hok := holdCallbacks.Load(event.Key)
			holdTrackerMu.Unlock()
			if !hok || hcb == nil {
				return false
			}
			util.Go(util.NewTraceContext(), fmt.Sprintf("hold hotkey release: %s", event.Key.Character()), func() {
				hcb.onRelease()
			})
			return false
		}

		holdTrackerMu.Unlock()
		return false
	})
	if err != nil {
		return err
	}

	holdKeyListener = listener
	return nil
}

// dispatchHoldModifierReleases invokes release callbacks after holdTrackerMu is unlocked.
func dispatchHoldModifierReleases(releases []holdModifierRelease) {
	for _, release := range releases {
		key := release.key
		callback := release.callback
		if callback == nil {
			continue
		}
		util.Go(util.NewTraceContext(), fmt.Sprintf("hold-modifier hotkey chord-cancel release: %s", key.Character()), func() {
			callback()
		})
	}
}

// armHoldModifierPress starts (or restarts) the minimum-hold timer for mcb.
// The caller must hold holdTrackerMu.
func armHoldModifierPress(mcb *holdModifierCallback, key keyboard.Key) {
	if mcb.pressTimer != nil {
		mcb.pressTimer.Stop()
	}
	mcb.pressFired = false
	mcb.pressTimer = time.AfterFunc(holdModifierPressDelay, func() {
		holdTrackerMu.Lock()
		if isOtherSpecificModifierPressed(key) {
			mcb.pressTimer = nil
			mcb.pressFired = false
			holdTrackerMu.Unlock()
			return
		}
		mcb.pressTimer = nil
		mcb.pressFired = true
		holdTrackerMu.Unlock()
		util.Go(util.NewTraceContext(), fmt.Sprintf("hold-modifier hotkey press: %s", key.Character()), func() {
			mcb.onPress()
		})
	})
}

// cancelHoldModifierPressesForChord cancels pending holds and returns release
// callbacks for already-fired holds. The except key is skipped so key-repeat on
// the active hold key can re-arm normally.
// The caller must hold holdTrackerMu.
func cancelHoldModifierPressesForChord(except keyboard.Key) ([]holdModifierRelease, bool) {
	releases := []holdModifierRelease{}
	canceled := false

	holdModifierCallbacks.Range(func(key keyboard.Key, mcb *holdModifierCallback) bool {
		if key == except || mcb == nil {
			return true
		}
		if mcb.pressTimer != nil {
			mcb.pressTimer.Stop()
			mcb.pressTimer = nil
			mcb.pressFired = false
			canceled = true
			return true
		}
		if mcb.pressFired {
			mcb.pressFired = false
			canceled = true
			if mcb.onRelease != nil {
				releases = append(releases, holdModifierRelease{key: key, callback: mcb.onRelease})
			}
		}
		return true
	})

	return releases, canceled
}

// isOtherSpecificModifierPressed catches simultaneous modifier chords whose
// raw key-down ordering would otherwise arm the hold key after the other
// modifier was already down.
func isOtherSpecificModifierPressed(activeKey keyboard.Key) bool {
	for key := range holdModifierRecorderKeys {
		if key == activeKey {
			continue
		}
		if keyboard.IsKeyPressed(key) {
			return true
		}
	}
	return false
}

// startHoldTracking begins watching for the release of the given key. When the
// key is released, onRelease is invoked. The callback persists across multiple
// press/release cycles. Multiple keys can be tracked simultaneously.
func startHoldTracking(key keyboard.Key, onRelease func()) error {
	holdTrackerMu.Lock()
	defer holdTrackerMu.Unlock()

	holdCallbacks.Store(key, &holdCallback{onRelease: onRelease, key: key})

	return ensureHoldKeyListener()
}

// stopHoldTracking removes the callbacks for the given key and closes the
// shared raw key listener when no more keys are being tracked.
func stopHoldTracking(key keyboard.Key) {
	holdTrackerMu.Lock()
	defer holdTrackerMu.Unlock()

	// Cancel any pending press timer for hold-modifier callbacks before removal.
	if mcb, ok := holdModifierCallbacks.Load(key); ok && mcb != nil {
		if mcb.pressTimer != nil {
			mcb.pressTimer.Stop()
			mcb.pressTimer = nil
		}
		mcb.pressFired = false
	}

	holdCallbacks.Delete(key)
	holdModifierCallbacks.Delete(key)

	if holdCallbacks.Len() > 0 || holdModifierCallbacks.Len() > 0 {
		return
	}

	if holdKeyListener != nil {
		_ = holdKeyListener.Close()
		holdKeyListener = nil
	}
}

// startHoldModifierTracking registers a hold-modifier hotkey that uses only a
// raw key listener for both press (key down) and release (key up). No system
// hotkey is registered, so there are no OS-level hotkey conflicts and left/right
// modifier keys can be distinguished.
//
// A minimum hold duration is enforced before onPress fires, and the action is
// suppressed if any other key is pressed during that window. This matches the
// "press and hold the modifier alone" semantics of hold mode.
func startHoldModifierTracking(key keyboard.Key, onPress func(), onRelease func()) error {
	holdTrackerMu.Lock()
	defer holdTrackerMu.Unlock()

	holdModifierCallbacks.Store(key, &holdModifierCallback{
		onPress:   onPress,
		onRelease: onRelease,
		key:       key,
	})

	return ensureHoldKeyListener()
}

// ---------------------------------------------------------------------------
// Hold-modifier recording
//
// Flutter's macOS engine does not reliably produce KeyDownEvent for every
// modifier key (notably right_ctrl), so the hold-hotkey recorder cannot rely
// on Flutter key events alone. When the UI enters hotkey-recording mode it
// installs a recorder callback via SetHoldModifierRecorder; the Go-side raw
// key listener (CGEventTap on macOS) captures the hold-modifier candidate
// keys and forwards the matched hold string back to the UI.
// ---------------------------------------------------------------------------

var (
	holdModifierRecorderMu       sync.Mutex
	holdModifierRecorder         func(string)
	holdModifierRecorderListener keyboard.RawKeySubscription
)

// holdModifierRecorderKeys are the keys the recorder will capture and forward.
var holdModifierRecorderKeys = map[keyboard.Key]bool{
	keyboard.KeyLeftCtrl:   true,
	keyboard.KeyRightCtrl:  true,
	keyboard.KeyLeftShift:  true,
	keyboard.KeyRightShift: true,
	keyboard.KeyLeftAlt:    true,
	keyboard.KeyRightAlt:   true,
	keyboard.KeyLeftSuper:  true,
	keyboard.KeyRightSuper: true,
}

// SetHoldModifierRecorder installs or removes a recorder that forwards
// hold-modifier key presses to the UI. When recorder is non-nil, a dedicated
// raw key listener is started; when nil, the listener is torn down. The
// listener is separate from the registered-hotkey listener so recording does
// not interfere with active hold-modifier hotkeys.
func SetHoldModifierRecorder(recorder func(string)) {
	var listenerToClose keyboard.RawKeySubscription

	holdModifierRecorderMu.Lock()
	holdModifierRecorder = recorder
	if recorder == nil && holdModifierRecorderListener != nil {
		listenerToClose = holdModifierRecorderListener
		holdModifierRecorderListener = nil
	}
	holdModifierRecorderMu.Unlock()

	if listenerToClose != nil {
		_ = listenerToClose.Close()
	}
	if recorder != nil {
		ensureHoldModifierRecorderListener()
	}
}

func ensureHoldModifierRecorderListener() {
	holdModifierRecorderMu.Lock()
	if holdModifierRecorderListener != nil {
		holdModifierRecorderMu.Unlock()
		return
	}
	holdModifierRecorderMu.Unlock()

	listener, err := keyboard.AddRawKeyListener(func(event keyboard.RawKeyEvent) bool {
		if event.Type != keyboard.EventTypeKeyDown {
			return false
		}
		if !holdModifierRecorderKeys[event.Key] {
			return false
		}

		holdModifierRecorderMu.Lock()
		rec := holdModifierRecorder
		holdModifierRecorderMu.Unlock()
		if rec == nil {
			return false
		}

		holdStr := event.Key.Character()
		if holdStr == "" {
			return false
		}
		util.Go(util.NewTraceContext(), fmt.Sprintf("record hold-modifier hotkey in UI: %s", holdStr), func() {
			rec(holdStr)
		})
		return false
	})
	if err != nil {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("failed to start hold-modifier recorder listener: %s", err.Error()))
		return
	}

	holdModifierRecorderMu.Lock()
	if holdModifierRecorderListener != nil {
		holdModifierRecorderMu.Unlock()
		_ = listener.Close()
		return
	}
	holdModifierRecorderListener = listener
	holdModifierRecorderMu.Unlock()
}
