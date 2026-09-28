package hotkey

import (
	"context"
	"runtime"
	"testing"
	"time"
	"wox/util/keyboard"
)

// TestWinModifierOwnersMaskRelease covers launcher, dictation press/hold, and
// double-Win bindings through the same raw hook contract.
func TestWinModifierOwnersMaskRelease(t *testing.T) {
	for _, mode := range []string{"launcher", "press", "hold", "double"} {
		t.Run(mode, func(t *testing.T) {
			rawHandler, restore := captureRawKeyListenerForTest(t)
			defer restore()
			hk := &Hotkey{}
			var err error
			switch mode {
			case "launcher":
				if runtime.GOOS != "windows" {
					t.Skip("single Win launcher shortcut is Windows-only")
				}
				err = hk.Register(context.Background(), "left_win", func() {})
			case "press":
				err = hk.RegisterWithModifierPress(context.Background(), "left_win", func() {})
			case "hold":
				err = hk.RegisterWithRelease(context.Background(), "left_win", func() {}, func() {})
			case "double":
				err = hk.Register(context.Background(), "win+win", func() {})
			}
			if err != nil {
				t.Fatal(err)
			}
			defer hk.Unregister(context.Background())
			for _, key := range []keyboard.Key{keyboard.KeyLeftSuper, keyboard.KeyRightSuper} {
				if rawHandler()(rawModifierEvent(keyboard.EventTypeKeyDown, key)) {
					t.Fatal("Win-down must reach the OS for combinations")
				}
				if rawHandler()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyE)) {
					t.Fatal("Win+E must reach the OS")
				}
				rawHandler()(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyE))
				wantMask := runtime.GOOS == "windows" && (key == keyboard.KeyLeftSuper || mode == "double")
				if masked := rawHandler()(rawModifierEvent(keyboard.EventTypeKeyUp, key)); masked != wantMask {
					t.Fatalf("Win-up masked=%t, want %t for %v", masked, wantMask, key)
				}
			}
			hk.Unregister(context.Background())
			if rawHandler()(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftSuper)) {
				t.Fatal("unregister must stop masking Win")
			}
		})
	}
}

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

// TestPartialWinChordKeepsStart verifies that registration alone does not own Win.
func TestPartialWinChordKeepsStart(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows release masking")
	}
	for _, hold := range []bool{false, true} {
		raw, restore := captureRawKeyListenerForTest(t)
		hk := &Hotkey{}
		var err error
		if hold {
			err = hk.RegisterWithRelease(context.Background(), "left_shift+left_win", func() {}, func() {})
		} else {
			err = hk.RegisterWithModifierPress(context.Background(), "left_shift+left_win", func() {})
		}
		if err != nil {
			restore()
			t.Fatal(err)
		}
		raw()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftSuper))
		if raw()(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftSuper)) {
			t.Error("unmatched chord suppressed Start")
		}
		raw()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftShift))
		raw()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftSuper))
		raw()(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftShift))
		if !raw()(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftSuper)) {
			t.Errorf("matched chord did not suppress Start: hold=%t pressed=%v", hold, pressModifierTracker.pressed)
		}
		raw()(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftShift))
		hk.Unregister(context.Background())
		restore()
	}
}

// TestDuplicateModifierGroupKeepsFirstOwner guards synced duplicate bindings.
func TestDuplicateModifierGroupKeepsFirstOwner(t *testing.T) {
	_, restore := captureRawKeyListenerForTest(t)
	defer restore()
	owner := ""
	group, err := RegisterGroup(context.Background(), []Spec{
		{CombineKey: "left_cmd", Callback: func() { owner = "main" }},
		{CombineKey: "left_cmd", Callback: func() { owner = "dictation" }},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer group.Unregister(context.Background())
	callback, ok := pressModifierCallbacks.Load("left_cmd")
	if !ok {
		t.Fatal("missing main callback")
	}
	callback()
	if owner != "main" || len(group.hotkeys) != 1 {
		t.Fatal("duplicate replaced first owner")
	}
}
