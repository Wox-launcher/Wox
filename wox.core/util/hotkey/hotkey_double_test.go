package hotkey

import (
	"testing"
	"wox/util/keyboard"
)

func TestDoubleTapTrackerTriggersOnlyOnPureModifierDoubleTap(t *testing.T) {
	tracker := newDoubleTapTracker()
	tracker.Register(keyboard.KeyCtrl)

	triggered := tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyCtrl), 100)
	assertNoTrigger(t, triggered)

	triggered = tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyCtrl), 120)
	assertNoTrigger(t, triggered)

	triggered = tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyCtrl), 200)
	assertNoTrigger(t, triggered)

	triggered = tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyCtrl), 220)
	assertTriggered(t, triggered, keyboard.KeyCtrl)
}

func TestDoubleTapTrackerRejectsInterveningNonModifierKey(t *testing.T) {
	tracker := newDoubleTapTracker()
	tracker.Register(keyboard.KeyCtrl)

	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyCtrl), 100)
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyCtrl), 120)

	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyCtrl), 200)
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeySpace), 210)
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeySpace), 220)

	triggered := tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyCtrl), 230)
	assertNoTrigger(t, triggered)
}

func TestDoubleTapTrackerRejectsDuplicateKeyUpWithoutNewKeyDown(t *testing.T) {
	tracker := newDoubleTapTracker()
	tracker.Register(keyboard.KeyCtrl)

	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyCtrl), 100)
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyCtrl), 120)
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyCtrl), 121)

	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyCtrl), 200)
	triggered := tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyCtrl), 220)
	assertTriggered(t, triggered, keyboard.KeyCtrl)

	triggered = tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyCtrl), 221)
	assertNoTrigger(t, triggered)
}

func TestDoubleTapTrackerRejectsInterveningOtherModifier(t *testing.T) {
	tracker := newDoubleTapTracker()
	tracker.Register(keyboard.KeyCtrl)

	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyCtrl), 100)
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyCtrl), 120)
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyShift), 180)
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyShift), 190)
	tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyCtrl), 220)

	triggered := tracker.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyCtrl), 240)
	assertNoTrigger(t, triggered)
}

func rawModifierEvent(eventType keyboard.EventType, key keyboard.Key) keyboard.RawKeyEvent {
	return keyboard.RawKeyEvent{
		Type: eventType,
		Key:  key,
	}
}

func assertNoTrigger(t *testing.T, triggered []keyboard.Key) {
	t.Helper()

	if len(triggered) != 0 {
		t.Fatalf("expected no trigger, got %v", triggered)
	}
}

func assertTriggered(t *testing.T, triggered []keyboard.Key, expected keyboard.Key) {
	t.Helper()

	if len(triggered) != 1 || triggered[0] != expected {
		t.Fatalf("expected %v trigger, got %v", expected, triggered)
	}
}
