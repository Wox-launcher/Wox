package hotkey

import (
	"fmt"
	"sync"
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
type holdModifierCallback struct {
	onPress   func()
	onRelease func()
	key       keyboard.Key
}

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
			// Check hold-modifier callbacks (press + release mode).
			mcb, mok := holdModifierCallbacks.Load(event.Key)
			holdTrackerMu.Unlock()
			if !mok || mcb == nil || mcb.onPress == nil {
				return false
			}
			util.Go(util.NewTraceContext(), fmt.Sprintf("hold-modifier hotkey press: %s", event.Key.Character()), func() {
				mcb.onPress()
			})
			return false
		}

		if event.Type == keyboard.EventTypeKeyUp {
			// Check hold-modifier callbacks first (press + release mode).
			mcb, mok := holdModifierCallbacks.Load(event.Key)
			if mok && mcb != nil {
				holdTrackerMu.Unlock()
				if mcb.onRelease != nil {
					util.Go(util.NewTraceContext(), fmt.Sprintf("hold-modifier hotkey release: %s", event.Key.Character()), func() {
						mcb.onRelease()
					})
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