package hotkey

import (
	"testing"
	"time"
	"wox/util/keyboard"
)

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

func captureRawKeyListenerForTest(t *testing.T) (func() keyboard.RawKeyHandler, func()) {
	t.Helper()

	var rawHandler keyboard.RawKeyHandler
	restore := replaceRawKeyListenerForTest(t, func(handler keyboard.RawKeyHandler) (keyboard.RawKeySubscription, error) {
		rawHandler = handler
		return noopRawKeySubscription{}, nil
	})
	return func() keyboard.RawKeyHandler {
		t.Helper()
		if rawHandler == nil {
			t.Fatalf("raw handler was not installed")
		}
		return rawHandler
	}, restore
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
