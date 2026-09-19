package hotkey

import (
	"runtime"
	"testing"
	"time"
	"wox/util/keyboard"
)

func TestHoldModifierKeysRelatedKeepsSidesDistinct(t *testing.T) {
	if !holdModifierKeysRelated(keyboard.KeyLeftShift, keyboard.KeyShift) {
		t.Fatal("generic shift should relate to left shift")
	}
	if holdModifierKeysRelated(keyboard.KeyLeftShift, keyboard.KeyRightShift) {
		t.Fatal("left shift must not relate to right shift")
	}
	if holdModifierKeysRelated(keyboard.KeyLeftCtrl, keyboard.KeyRightCtrl) {
		t.Fatal("left ctrl must not relate to right ctrl")
	}
}

func TestHoldModifierTrackingFiresPressAndReleaseForSingleKey(t *testing.T) {
	rawHandler, restore := captureRawKeyListenerForTest(t)
	defer restore()

	pressed := make(chan struct{}, 1)
	released := make(chan struct{}, 1)
	if err := startHoldModifierTracking([]keyboard.Key{keyboard.KeyLeftAlt}, func() {
		pressed <- struct{}{}
	}, func() {
		released <- struct{}{}
	}); err != nil {
		t.Fatalf("start hold modifier tracking: %v", err)
	}
	defer stopHoldModifierTracking([]keyboard.Key{keyboard.KeyLeftAlt})

	rawHandler()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftAlt))
	assertSignal(t, pressed, "hold press")

	rawHandler()(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftAlt))
	assertSignal(t, released, "hold release")
}

func TestHoldModifierTrackingRequiresWholeChordAndReleasesOnAnyChordKey(t *testing.T) {
	rawHandler, restore := captureRawKeyListenerForTest(t)
	defer restore()

	pressed := make(chan struct{}, 1)
	released := make(chan struct{}, 1)
	keys := []keyboard.Key{keyboard.KeyLeftShift, keyboard.KeyLeftSuper}
	if err := startHoldModifierTracking(keys, func() {
		pressed <- struct{}{}
	}, func() {
		released <- struct{}{}
	}); err != nil {
		t.Fatalf("start hold modifier tracking: %v", err)
	}
	defer stopHoldModifierTracking(keys)

	rawHandler()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftShift))
	assertNoSignal(t, pressed, "partial hold press")

	rawHandler()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftSuper))
	assertSignal(t, pressed, "chord hold press")

	rawHandler()(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftShift))
	assertSignal(t, released, "chord hold release")
}

func TestHoldModifierTrackingCancelsWhenExtraKeyPressed(t *testing.T) {
	rawHandler, restore := captureRawKeyListenerForTest(t)
	defer restore()

	pressed := make(chan struct{}, 1)
	released := make(chan struct{}, 1)
	keys := []keyboard.Key{keyboard.KeyLeftAlt}
	if err := startHoldModifierTracking(keys, func() {
		pressed <- struct{}{}
	}, func() {
		released <- struct{}{}
	}); err != nil {
		t.Fatalf("start hold modifier tracking: %v", err)
	}
	defer stopHoldModifierTracking(keys)

	rawHandler()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftAlt))
	rawHandler()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeySpace))
	assertNoSignal(t, pressed, "canceled hold press")

	rawHandler()(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftAlt))
	assertNoSignal(t, released, "canceled hold release")
}

func TestHoldModifierTrackingUnregisterCancelsPendingPress(t *testing.T) {
	rawHandler, restore := captureRawKeyListenerForTest(t)
	defer restore()

	pressed := make(chan struct{}, 1)
	keys := []keyboard.Key{keyboard.KeyLeftAlt}
	if err := startHoldModifierTracking(keys, func() {
		pressed <- struct{}{}
	}, nil); err != nil {
		t.Fatalf("start hold modifier tracking: %v", err)
	}

	rawHandler()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftAlt))
	stopHoldModifierTracking(keys)
	assertNoSignal(t, pressed, "unregistered hold press")
}

func TestHoldModifierTrackingAcceptsGenericFamilyKey(t *testing.T) {
	rawHandler, restore := captureRawKeyListenerForTest(t)
	defer restore()

	pressed := make(chan struct{}, 1)
	keys := []keyboard.Key{keyboard.KeyLeftCtrl, keyboard.KeyLeftShift}
	if err := startHoldModifierTracking(keys, func() {
		pressed <- struct{}{}
	}, nil); err != nil {
		t.Fatalf("start hold modifier tracking: %v", err)
	}
	defer stopHoldModifierTracking(keys)

	rawHandler()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftCtrl))
	assertNoSignal(t, pressed, "partial hold press")

	rawHandler()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyShift))
	assertSignal(t, pressed, "generic shift hold press")
}

func TestHoldModifierTrackingKeepsLeftAndRightShiftDistinct(t *testing.T) {
	rawHandler, restore := captureRawKeyListenerForTest(t)
	defer restore()

	pressed := make(chan struct{}, 1)
	keys := []keyboard.Key{keyboard.KeyLeftCtrl, keyboard.KeyLeftShift}
	if err := startHoldModifierTracking(keys, func() {
		pressed <- struct{}{}
	}, nil); err != nil {
		t.Fatalf("start hold modifier tracking: %v", err)
	}
	defer stopHoldModifierTracking(keys)

	rawHandler()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftCtrl))
	rawHandler()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyRightShift))
	assertNoSignal(t, pressed, "right shift should not satisfy left shift")
}

func TestHoldModifierTrackingClearsStuckModifierFromOS(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("physical modifier reconciliation is Windows-only")
	}
	rawHandler, restore := captureRawKeyListenerForTest(t)
	defer restore()

	isHoldModifierPhysicallyPressed = func(key keyboard.Key) bool {
		return key != keyboard.KeyLeftSuper && holdModifierPressed[key]
	}

	holdTrackerMu.Lock()
	holdModifierPressed[keyboard.KeyLeftSuper] = true
	holdTrackerMu.Unlock()

	pressed := make(chan struct{}, 1)
	keys := []keyboard.Key{keyboard.KeyLeftCtrl, keyboard.KeyLeftShift}
	if err := startHoldModifierTracking(keys, func() {
		pressed <- struct{}{}
	}, nil); err != nil {
		t.Fatalf("start hold modifier tracking: %v", err)
	}
	defer stopHoldModifierTracking(keys)

	rawHandler()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftCtrl))
	rawHandler()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftShift))
	assertSignal(t, pressed, "hold press after clearing stuck win key")
}

// TestHoldModifierReconciliationPlatformBoundary preserves event state on
// platforms where the physical query does not represent every keyboard.
func TestHoldModifierReconciliationPlatformBoundary(t *testing.T) {
	_, restore := captureRawKeyListenerForTest(t)
	defer restore()

	queries := 0
	isHoldModifierPhysicallyPressed = func(key keyboard.Key) bool {
		queries++
		return false
	}
	holdTrackerMu.Lock()
	defer holdTrackerMu.Unlock()
	holdModifierPressed[keyboard.KeyLeftCtrl] = true
	holdModifierPressed[keyboard.KeyLeftShift] = true
	reconcileStuckHoldModifiers(keyboard.KeyLeftShift)

	if !holdModifierPressed[keyboard.KeyLeftShift] {
		t.Fatal("the current hook event must not be invalidated by physical state")
	}
	if runtime.GOOS == "windows" {
		if holdModifierPressed[keyboard.KeyLeftCtrl] || queries != 1 {
			t.Fatal("Windows should query and clear the stale modifier")
		}
	} else if !holdModifierPressed[keyboard.KeyLeftCtrl] || queries != 0 {
		t.Fatal("other platforms must preserve raw event state without physical queries")
	}
}

func captureRawKeyListenerForTest(t *testing.T) (func() keyboard.RawKeyHandler, func()) {
	t.Helper()

	var rawHandler keyboard.RawKeyHandler
	restoreListener := replaceRawKeyListenerForTest(t, func(handler keyboard.RawKeyHandler) (keyboard.RawKeySubscription, error) {
		rawHandler = handler
		return noopRawKeySubscription{}, nil
	})
	previousPhysical := isHoldModifierPhysicallyPressed
	isHoldModifierPhysicallyPressed = func(key keyboard.Key) bool {
		return holdModifierPressed[key]
	}
	holdTrackerMu.Lock()
	holdModifierPressed = map[keyboard.Key]bool{}
	holdTrackerMu.Unlock()
	return func() keyboard.RawKeyHandler {
			t.Helper()
			if rawHandler == nil {
				t.Fatalf("raw handler was not installed")
			}
			return rawHandler
		}, func() {
			isHoldModifierPhysicallyPressed = previousPhysical
			holdTrackerMu.Lock()
			holdModifierPressed = map[keyboard.Key]bool{}
			holdTrackerMu.Unlock()
			restoreListener()
		}
}

func assertSignal(t *testing.T, ch <-chan struct{}, label string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(holdModifierPressDelay + 800*time.Millisecond):
		t.Fatalf("expected %s", label)
	}
}

func assertNoSignal(t *testing.T, ch <-chan struct{}, label string) {
	t.Helper()

	select {
	case <-ch:
		t.Fatalf("did not expect %s", label)
	case <-time.After(holdModifierPressDelay + 80*time.Millisecond):
	}
}
