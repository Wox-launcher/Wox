package hotkey

import (
	"runtime"
	"testing"
	"wox/util/keyboard"
)

// TestModifierPressTrackerRecoversLostWindowsReleases keeps Win+L from
// leaving the launcher's single-Win shortcut canceled after unlocking.
func TestModifierPressTrackerRecoversLostWindowsReleases(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native state reconciliation is Windows-only")
	}
	previous := isPressModifierPhysicallyPressed
	t.Cleanup(func() { isPressModifierPhysicallyPressed = previous })
	isPressModifierPhysicallyPressed = func(keyboard.Key) bool { return false }
	tracker := newModifierPressTracker()
	tracker.Register([]keyboard.Key{keyboard.KeyLeftSuper})
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftSuper), neverDelayModifierPress, 100)
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyL), neverDelayModifierPress, 110)
	tracker.HandleEvent(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyLeftSuper, NativeKeyCode: 0x5B}, neverDelayModifierPress, 200)
	assertModifierPress(t, tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftSuper), neverDelayModifierPress, 210), "left_cmd")
}

func TestModifierPressTrackerTriggersSingleModifierOnPurePress(t *testing.T) {
	tracker := newModifierPressTracker()
	tracker.Register([]keyboard.Key{keyboard.KeyLeftAlt})

	triggered := tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftAlt), neverDelayModifierPress, 100)
	assertNoModifierPress(t, triggered)

	triggered = tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftAlt), neverDelayModifierPress, 120)
	assertModifierPress(t, triggered, "left_alt")
}

func TestModifierPressTrackerTriggersTwoModifierChordAfterBothRelease(t *testing.T) {
	tracker := newModifierPressTracker()
	tracker.Register([]keyboard.Key{keyboard.KeyLeftShift, keyboard.KeyLeftSuper})

	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftShift), neverDelayModifierPress, 100)
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftSuper), neverDelayModifierPress, 110)
	triggered := tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftShift), neverDelayModifierPress, 120)
	assertNoModifierPress(t, triggered)

	triggered = tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftSuper), neverDelayModifierPress, 130)
	assertModifierPress(t, triggered, "left_shift+left_cmd")
}

func TestModifierPressTrackerCancelsWhenExtraKeyPressed(t *testing.T) {
	tracker := newModifierPressTracker()
	tracker.Register([]keyboard.Key{keyboard.KeyLeftAlt})

	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftAlt), neverDelayModifierPress, 100)
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeySpace), neverDelayModifierPress, 110)
	triggered := tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftAlt), neverDelayModifierPress, 120)
	assertNoModifierPress(t, triggered)
}

// TestModifierPressTrackerRejectsOtherHeldAndUnknownKeys covers combinations
// whose extra key arrives before Win or has no portable key name.
func TestModifierPressTrackerRejectsOtherHeldAndUnknownKeys(t *testing.T) {
	for _, extra := range []keyboard.Key{keyboard.KeyE, keyboard.KeyUnknown} {
		for _, before := range []bool{true, false} {
			tracker := newModifierPressTracker()
			tracker.Register([]keyboard.Key{keyboard.KeyLeftSuper})
			if before {
				tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, extra), neverDelayModifierPress, 90)
			}
			tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftSuper), neverDelayModifierPress, 100)
			if !before {
				tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, extra), neverDelayModifierPress, 110)
			}
			tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, extra), neverDelayModifierPress, 120)
			// Repeated Win-down must not revive a canceled combination.
			tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftSuper), neverDelayModifierPress, 125)
			assertNoModifierPress(t, tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftSuper), neverDelayModifierPress, 130))
			tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftSuper), neverDelayModifierPress, 140)
			assertModifierPress(t, tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftSuper), neverDelayModifierPress, 150), "left_cmd")
		}
	}
}

func TestModifierPressTrackerDelaysSingleModifierPressWhenDoubleModifierCanMatch(t *testing.T) {
	tracker := newModifierPressTracker()
	tracker.Register([]keyboard.Key{keyboard.KeyLeftCtrl})

	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftCtrl), delayCtrlModifierPress, 100)
	triggered := tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftCtrl), delayCtrlModifierPress, 120)
	assertNoModifierPress(t, triggered)

	triggered = tracker.FlushDelayed(619)
	assertNoModifierPress(t, triggered)

	triggered = tracker.FlushDelayed(620)
	assertModifierPress(t, triggered, "left_ctrl")
}

func TestModifierPressTrackerCancelsDelayedPressWhenSecondPressStarts(t *testing.T) {
	tracker := newModifierPressTracker()
	tracker.Register([]keyboard.Key{keyboard.KeyLeftCtrl})

	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftCtrl), delayCtrlModifierPress, 100)
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftCtrl), delayCtrlModifierPress, 120)
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftCtrl), delayCtrlModifierPress, 200)

	triggered := tracker.FlushDelayed(620)
	assertNoModifierPress(t, triggered)
}

func TestModifierPressTrackerSuppressesPressWhenDoubleModifierAlreadyTriggered(t *testing.T) {
	tracker := newModifierPressTracker()
	tracker.Register([]keyboard.Key{keyboard.KeyLeftCtrl})

	tracker.SuppressNextPressForRawKey(keyboard.KeyLeftCtrl, 200)
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftCtrl), delayCtrlModifierPress, 201)
	triggered := tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftCtrl), delayCtrlModifierPress, 220)
	assertNoModifierPress(t, triggered)

	triggered = tracker.FlushDelayed(720)
	assertNoModifierPress(t, triggered)
}

func neverDelayModifierPress(key keyboard.Key) bool {
	return false
}

func delayCtrlModifierPress(key keyboard.Key) bool {
	return modifierKeyMatchesRawEvent(keyboard.KeyCtrl, key)
}

func assertNoModifierPress(t *testing.T, triggered []modifierPressTrigger) {
	t.Helper()

	if len(triggered) != 0 {
		t.Fatalf("expected no modifier press, got %v", triggered)
	}
}

func assertModifierPress(t *testing.T, triggered []modifierPressTrigger, expected string) {
	t.Helper()

	if len(triggered) != 1 || triggered[0].combo != expected {
		t.Fatalf("expected %s modifier press, got %v", expected, triggered)
	}
}

// TestUnknownNativeKeys verifies independent releases and recovery after a lost release.
func TestUnknownNativeKeys(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows reconciliation")
	}
	previous := isPressNativeKeyPressed
	defer func() { isPressNativeKeyPressed = previous }()
	held := map[uint32]bool{0x24: true, 0x23: true}
	isPressNativeKeyPressed = func(code uint32) bool { return held[code] }
	tracker := newModifierPressTracker()
	tracker.Register([]keyboard.Key{keyboard.KeyLeftSuper})
	for _, code := range []uint32{0x24, 0x23} {
		tracker.HandleEvent(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyUnknown, NativeKeyCode: code}, neverDelayModifierPress, 100)
	}
	delete(held, 0x23)
	tracker.HandleEvent(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyUp, Key: keyboard.KeyUnknown, NativeKeyCode: 0x23}, neverDelayModifierPress, 110)
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftSuper), neverDelayModifierPress, 120)
	assertNoModifierPress(t, tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftSuper), neverDelayModifierPress, 130))
	delete(held, 0x24)
	tracker.HandleEvent(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyLeftSuper, NativeKeyCode: 0x5B}, neverDelayModifierPress, 140)
	assertModifierPress(t, tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftSuper), neverDelayModifierPress, 150), "left_cmd")
}
