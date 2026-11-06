package hotkey

import (
	"context"
	"testing"
	"time"
	"wox/util/keyboard"
)

// TestStandaloneHoldTapAndHold checks replay, auto-repeat, and lifecycle ordering.
func TestStandaloneHoldTapAndHold(t *testing.T) {
	replayed := make(chan uint32, 2)
	transitions := make(chan HoldTransition, 2)
	h := &standaloneHold{key: keyboard.KeyDelete, replay: func(code uint32) error {
		replayed <- code
		return nil
	}, onTransition: func(event HoldTransition) { transitions <- event }}
	down := keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyDelete, NativeKeyCode: 46}
	up := keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyUp, Key: keyboard.KeyDelete, NativeKeyCode: 46}
	if !h.handle(down) || !h.handle(up) {
		t.Fatal("short press was not consumed")
	}
	select {
	case code := <-replayed:
		if code != 46 {
			t.Fatalf("replayed code = %d", code)
		}
	case <-time.After(time.Second):
		t.Fatal("short press was not replayed")
	}
	if !h.handle(down) || !h.handle(down) {
		t.Fatal("hold or repeat was not consumed")
	}
	select {
	case event := <-transitions:
		if !event.Started {
			t.Fatalf("first transition = %+v", event)
		}
	case <-time.After(holdPressDelay + time.Second):
		t.Fatal("hold did not start")
	}
	if !h.handle(up) {
		t.Fatal("hold release was not consumed")
	}
	select {
	case event := <-transitions:
		if event.Started || event.Reason != HoldEndReleased {
			t.Fatalf("release = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("hold did not end")
	}
	select {
	case code := <-replayed:
		t.Fatalf("hold replayed code %d", code)
	default:
	}
}

// TestStandaloneHoldGroupRegistersAndUnregisters verifies the plugin registration path.
func TestStandaloneHoldGroupRegistersAndUnregisters(t *testing.T) {
	if !keyboard.SupportsStandaloneHold(keyboard.KeyDelete) {
		t.Skip("standalone hold interception unavailable")
	}
	raw, restore := captureRawKeyListenerForTest(t)
	defer restore()
	group, err := RegisterGroup(context.Background(), []Spec{{CombineKey: "delete", Callback: func() {}, OnRelease: func() {}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(group.RegisteredCombineKeys()) != 1 || raw() == nil {
		t.Fatal("standalone hold was not registered")
	}
	group.Unregister(context.Background())
}

// TestStandaloneHoldPreservesModifiedKeysAndInterruptedTaps checks non-hold input.
func TestStandaloneHoldPreservesModifiedKeysAndInterruptedTaps(t *testing.T) {
	count := 0
	h := &standaloneHold{key: keyboard.KeyDelete, replay: func(uint32) error { count++; return nil }}
	down := keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyDelete, NativeKeyCode: 46}
	modified := down
	modified.Modifiers = keyboard.ModifierCtrl
	if h.handle(modified) || h.handle(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyUp, Key: keyboard.KeyDelete}) {
		t.Fatal("modified Delete was consumed")
	}
	h.handle(down)
	h.handle(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyA})
	h.handle(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyUp, Key: keyboard.KeyDelete})
	if count != 1 {
		t.Fatalf("interrupted tap replays = %d", count)
	}
}

// TestStandaloneHoldCloseRestoresPendingTapAndIgnoresLateEvents covers teardown.
func TestStandaloneHoldCloseRestoresPendingTapAndIgnoresLateEvents(t *testing.T) {
	count := 0
	h := &standaloneHold{key: keyboard.KeyInsert, replay: func(uint32) error { count++; return nil }}
	down := keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyInsert}
	if !h.handle(down) {
		t.Fatal("pending tap was not consumed")
	}
	h.close()
	if count != 1 || h.handle(down) {
		t.Fatalf("replays=%d or listener accepted input after close", count)
	}
}

// TestStandaloneHoldDoesNotReplayIntoAnotherTarget protects input after a focus change.
func TestStandaloneHoldDoesNotReplayIntoAnotherTarget(t *testing.T) {
	target := uint64(1)
	count := 0
	h := &standaloneHold{key: keyboard.KeyDelete, inputTarget: func() uint64 { return target }, replay: func(uint32) error { count++; return nil }}
	h.handle(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyDelete})
	target = 2
	h.handle(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyUp, Key: keyboard.KeyDelete})
	if count != 0 {
		t.Fatal("short press was replayed into a different target")
	}
}

// TestStandaloneHoldReplaysEachSupportedKey verifies the full standalone set.
func TestStandaloneHoldReplaysEachSupportedKey(t *testing.T) {
	keys := []keyboard.Key{keyboard.KeyDelete, keyboard.KeyInsert}
	for key := keyboard.KeyF1; key <= keyboard.KeyF24; key++ {
		keys = append(keys, key)
	}
	for _, key := range keys {
		count := 0
		h := &standaloneHold{key: key, replay: func(uint32) error { count++; return nil }}
		if !h.handle(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: key}) || !h.handle(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyUp, Key: key}) || count != 1 {
			t.Fatalf("key %v did not replay its short press", key)
		}
	}
}
