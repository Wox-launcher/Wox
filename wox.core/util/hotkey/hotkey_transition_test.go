package hotkey

import (
	"testing"
	"time"
	"wox/util/keyboard"
)

// TestHoldTransitionOrdering exercises the raw hook, hold timer and unregister path.
func TestHoldTransitionOrdering(t *testing.T) {
	for _, ending := range []HoldEndReason{HoldEndReleased, HoldEndInterrupted, HoldEndUnavailable} {
		t.Run(string(ending), func(t *testing.T) {
			raw, restore := captureRawKeyListenerForTest(t)
			defer restore()
			transitions := make(chan HoldTransition, 4)
			hk := &Hotkey{}
			err := hk.register(t.Context(), "right_alt", func() {}, func() {}, registerOptions{onHoldTransition: func(event HoldTransition) { transitions <- event }})
			if err != nil {
				t.Fatal(err)
			}
			defer hk.Unregister(t.Context())
			raw()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyRightAlt))
			select {
			case e := <-transitions:
				if !e.Started {
					t.Fatal(e)
				}
			case <-time.After(time.Second):
				t.Fatal("hold never started")
			}
			switch ending {
			case HoldEndReleased:
				raw()(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyRightAlt))
			case HoldEndInterrupted:
				raw()(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyE))
			case HoldEndUnavailable:
				hk.Unregister(t.Context())
			}
			select {
			case e := <-transitions:
				if e.Started || e.Reason != ending {
					t.Fatal(e)
				}
			case <-time.After(time.Second):
				t.Fatal("missing end")
			}
			raw()(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyRightAlt))
			select {
			case e := <-transitions:
				t.Fatalf("duplicate end: %+v", e)
			default:
			}
		})
	}
}
