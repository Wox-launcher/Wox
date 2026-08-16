//go:build windows

package explorer

import (
	"testing"
	"wox/util/keyboard"
)

func TestTypeToSearchPreservesExplorerFunctionKeys(t *testing.T) {
	for _, key := range []keyboard.Key{
		keyboard.KeyF2,
		keyboard.KeyF3,
		keyboard.KeyF4,
		keyboard.KeyF5,
		keyboard.KeyF6,
		keyboard.KeyF10,
		keyboard.KeyF11,
	} {
		event := keyboard.RawKeyEvent{
			Type:      keyboard.EventTypeKeyDown,
			Key:       key,
			Character: key.Character(),
		}
		if shouldDispatchTypeToSearch(event) {
			t.Fatalf("%s triggered type-to-search", key.Character())
		}
	}
}

func TestTypeToSearchAcceptsUnassignedFunctionKey(t *testing.T) {
	event := keyboard.RawKeyEvent{
		Type:      keyboard.EventTypeKeyDown,
		Key:       keyboard.KeyF7,
		Character: "f7",
	}
	if !shouldDispatchTypeToSearch(event) {
		t.Fatal("F7 did not trigger type-to-search")
	}
}
