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
//   - pressTimer != nil, pressFired == false: all keys in the chord are held
//     but the minimum hold duration has not yet elapsed. A quick press or any
//     key press outside the chord cancels the timer and suppresses both callbacks.
//   - pressTimer == nil, pressFired == true: the chord was held long enough and
//     onPress has fired. The next key-up for any chord key triggers onRelease.
//     Any key-down outside the chord first ends the hold immediately and
//     suppresses the later key-up release callback.
//
// This ensures the action only fires when the exact modifier chord is held for
// the full hold duration. Pressing another key (e.g. space while holding cmd
// for cmd+space) cancels the action.
type holdModifierCallback struct {
	onPress    func()
	onRelease  func()
	keys       []keyboard.Key
	combo      string
	pressTimer *time.Timer
	pressFired bool
	pressSeq   int64
}

type holdModifierRelease struct {
	combo    string
	callback func()
}

// holdModifierPressDelay is the minimum time a hold-modifier key must remain
// pressed alone before onPress fires. Presses shorter than this, or holds
// interrupted by another key press, are treated as accidental and ignored,
// matching the "press and hold" semantics users expect from hold mode.
const holdModifierPressDelay = 200 * time.Millisecond

var (
	holdCallbacks         = util.NewHashMap[keyboard.Key, *holdCallback]()
	holdModifierCallbacks = util.NewHashMap[string, *holdModifierCallback]()
	holdModifierPressed   = map[keyboard.Key]bool{}
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

	listener, err := addRawKeyListener(func(event keyboard.RawKeyEvent) bool {
		handlePressModifierRawEvent(event)

		holdTrackerMu.Lock()

		if event.Type == keyboard.EventTypeKeyDown {
			if holdModifierRecorderKeys[event.Key] {
				holdModifierPressed[event.Key] = true
			}
			releases := cancelHoldModifierPressesForExternalKey(event.Key)
			callbacks := holdModifierCallbacksForKey(event.Key)
			for _, mcb := range callbacks {
				if mcb == nil || !holdModifierExactKeysPressed(mcb.keys) {
					continue
				}
				util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf("hold-modifier keyDown: combo=%s timer=%v fired=%v", mcb.combo, mcb.pressTimer != nil, mcb.pressFired))
				armHoldModifierPress(mcb)
			}
			holdTrackerMu.Unlock()
			dispatchHoldModifierReleases(releases)
			return false
		}

		if event.Type == keyboard.EventTypeKeyUp {
			if holdModifierRecorderKeys[event.Key] {
				holdModifierPressed[event.Key] = false
			}
			// Check hold-modifier callbacks first (press + release mode).
			releases := releaseHoldModifierPressesForKey(event.Key)
			if releases != nil {
				holdTrackerMu.Unlock()
				dispatchHoldModifierReleases(releases)
				return false
			}

			// Fall back to release-only hold callbacks.
			callbacks := holdCallbacksForRawKey(event.Key)
			holdTrackerMu.Unlock()
			if len(callbacks) == 0 {
				return false
			}
			for _, hcb := range callbacks {
				hcb := hcb
				util.Go(util.NewTraceContext(), fmt.Sprintf("hold hotkey release: %s", modifierKeyLogLabel(hcb.key)), func() {
					hcb.onRelease()
				})
			}
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
		combo := release.combo
		callback := release.callback
		if callback == nil {
			continue
		}
		util.Go(util.NewTraceContext(), fmt.Sprintf("hold-modifier hotkey release: %s", combo), func() {
			callback()
		})
	}
}

// armHoldModifierPress starts the minimum-hold timer for mcb.
// The caller must hold holdTrackerMu.
func armHoldModifierPress(mcb *holdModifierCallback) {
	if mcb.pressTimer != nil || mcb.pressFired {
		return
	}
	mcb.pressSeq++
	pressSeq := mcb.pressSeq
	mcb.pressTimer = time.AfterFunc(holdModifierPressDelay, func() {
		holdTrackerMu.Lock()
		if mcb.pressSeq != pressSeq || !holdModifierExactKeysPressed(mcb.keys) {
			mcb.pressTimer = nil
			mcb.pressFired = false
			holdTrackerMu.Unlock()
			return
		}
		mcb.pressTimer = nil
		mcb.pressFired = true
		combo := mcb.combo
		onPress := mcb.onPress
		holdTrackerMu.Unlock()
		if onPress == nil {
			return
		}
		util.Go(util.NewTraceContext(), fmt.Sprintf("hold-modifier hotkey press: %s", combo), func() {
			onPress()
		})
	})
}

// cancelHoldModifierPressesForExternalKey cancels hold-modifier callbacks whose
// exact chord does not include key. The caller must hold holdTrackerMu.
func cancelHoldModifierPressesForExternalKey(key keyboard.Key) []holdModifierRelease {
	releases := []holdModifierRelease{}

	holdModifierCallbacks.Range(func(_ string, mcb *holdModifierCallback) bool {
		if mcb == nil {
			return true
		}
		if key != keyboard.KeyUnknown && containsHoldModifierKey(mcb.keys, key) {
			return true
		}
		releases = append(releases, resetHoldModifierCallback(mcb)...)
		return true
	})

	return releases
}

// releaseHoldModifierPressesForKey ends every hold-modifier callback that uses
// key. A nil result means the key was not part of any hold-modifier callback.
// The caller must hold holdTrackerMu.
func releaseHoldModifierPressesForKey(key keyboard.Key) []holdModifierRelease {
	releases := []holdModifierRelease{}
	matched := false

	holdModifierCallbacks.Range(func(_ string, mcb *holdModifierCallback) bool {
		if mcb == nil || !containsHoldModifierKey(mcb.keys, key) {
			return true
		}
		matched = true
		util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf("hold-modifier keyUp: combo=%s timer=%v fired=%v", mcb.combo, mcb.pressTimer != nil, mcb.pressFired))
		releases = append(releases, resetHoldModifierCallback(mcb)...)
		return true
	})

	if !matched {
		return nil
	}
	return releases
}

// resetHoldModifierCallback stops a pending hold or prepares a release callback
// for an already-fired hold. The caller must hold holdTrackerMu.
func resetHoldModifierCallback(mcb *holdModifierCallback) []holdModifierRelease {
	if mcb.pressTimer != nil {
		mcb.pressTimer.Stop()
		mcb.pressTimer = nil
		mcb.pressFired = false
		mcb.pressSeq++
		return nil
	}
	if mcb.pressFired {
		mcb.pressFired = false
		mcb.pressSeq++
		if mcb.onRelease != nil {
			return []holdModifierRelease{{combo: mcb.combo, callback: mcb.onRelease}}
		}
	}
	return nil
}

// holdModifierCallbacksForKey returns callbacks whose exact chord contains key.
// The caller must hold holdTrackerMu.
func holdModifierCallbacksForKey(key keyboard.Key) []*holdModifierCallback {
	callbacks := []*holdModifierCallback{}
	holdModifierCallbacks.Range(func(_ string, mcb *holdModifierCallback) bool {
		if mcb != nil && containsHoldModifierKey(mcb.keys, key) {
			callbacks = append(callbacks, mcb)
		}
		return true
	})
	return callbacks
}

// holdCallbacksForRawKey returns release-only callbacks that match a raw key event.
// The caller must hold holdTrackerMu.
func holdCallbacksForRawKey(key keyboard.Key) []*holdCallback {
	callbacks := []*holdCallback{}
	holdCallbacks.Range(func(registeredKey keyboard.Key, hcb *holdCallback) bool {
		if hcb != nil && modifierKeyMatchesRawEvent(registeredKey, key) {
			callbacks = append(callbacks, hcb)
		}
		return true
	})
	return callbacks
}

// holdModifierExactKeysPressed verifies that every key in the chord is down and
// no other specific modifier is currently held.
// The caller must hold holdTrackerMu.
func holdModifierExactKeysPressed(keys []keyboard.Key) bool {
	if len(keys) == 0 {
		return false
	}
	for _, key := range keys {
		if !holdModifierPressed[key] {
			return false
		}
	}
	for key := range holdModifierRecorderKeys {
		if containsHoldModifierKey(keys, key) {
			continue
		}
		if holdModifierPressed[key] {
			return false
		}
	}
	return true
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

	holdCallbacks.Delete(key)
	closeHoldKeyListenerIfIdle()
}

// stopHoldModifierTracking removes a hold-modifier chord callback and closes
// the shared raw key listener when no more keys are being tracked.
func stopHoldModifierTracking(keys []keyboard.Key) {
	holdTrackerMu.Lock()
	defer holdTrackerMu.Unlock()

	combo := holdModifierComboString(keys)
	if mcb, ok := holdModifierCallbacks.Load(combo); ok && mcb != nil {
		resetHoldModifierCallback(mcb)
	}
	holdModifierCallbacks.Delete(combo)
	closeHoldKeyListenerIfIdle()
}

func closeHoldKeyListenerIfIdle() {
	if holdCallbacks.Len() > 0 || holdModifierCallbacks.Len() > 0 || pressModifierTracker.Len() > 0 {
		return
	}
	if holdKeyListener != nil {
		_ = holdKeyListener.Close()
		holdKeyListener = nil
	}
	holdModifierPressed = map[keyboard.Key]bool{}
}

// startHoldModifierTracking registers a hold-modifier chord that uses only a
// raw key listener for both press (key down) and release (key up). No system
// hotkey is registered, so there are no OS-level hotkey conflicts and
// left/right modifier keys can be distinguished.
//
// A minimum hold duration is enforced before onPress fires, and the action is
// suppressed if any key outside the chord is pressed during that window.
func startHoldModifierTracking(keys []keyboard.Key, onPress func(), onRelease func()) error {
	holdTrackerMu.Lock()
	defer holdTrackerMu.Unlock()

	canonicalKeys := canonicalHoldModifierKeys(keys)
	combo := holdModifierComboString(canonicalKeys)
	holdModifierCallbacks.Store(combo, &holdModifierCallback{
		onPress:   onPress,
		onRelease: onRelease,
		keys:      canonicalKeys,
		combo:     combo,
	})

	return ensureHoldKeyListener()
}

// holdModifierRecorderKeys are the left/right specific modifier keys tracked
// by holdModifier and pressModifier runtime hotkeys.
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
