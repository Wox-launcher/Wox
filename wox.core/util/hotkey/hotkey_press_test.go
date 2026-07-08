package hotkey

import (
	"testing"
	"wox/util/keyboard"
)

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
