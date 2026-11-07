package snippets

import (
	"testing"

	"wox/util/keyboard"
)

// TestWindowsInputTranslation checks dead-key invalidation and injection filtering before matching.
func TestWindowsInputTranslation(t *testing.T) {
	var observer windowsInputObserver
	translate := func(value uint32) func(uint32, uint32) uint32 {
		return func(uint32, uint32) uint32 { return value }
	}
	key := keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyA, NativeKeyCode: 65}
	if event, ok := observer.translate(key, 42, translate('a')); !ok || event.kind != inputText || event.text != "a" || event.target != 42 {
		t.Fatalf("plain key: %+v %v", event, ok)
	}
	key.NativeFlags = 0x10
	key.NativeExtraInfo = windowsSnippetInputTag
	if _, ok := observer.translate(key, 42, translate('a')); ok {
		t.Fatal("injected key entered matching")
	}
	key.NativeExtraInfo = 0
	if event, ok := observer.translate(key, 42, translate('a')); !ok || event.kind != inputInvalidate || event.target != 0 {
		t.Fatal("other injected input must cancel pending work")
	}
	key.NativeFlags = 0
	if event, _ := observer.translate(key, 42, translate(0xffffffff)); event.kind != inputInvalidate {
		t.Fatal("dead key did not invalidate")
	}
	modifier := keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyLeftShift}
	if event, _ := observer.translate(modifier, 42, translate(0)); event.kind != inputModifier {
		t.Fatal("shift was not a modifier")
	}
	if event, _ := observer.translate(key, 42, translate('a')); event.kind != inputInvalidate {
		t.Fatal("composed key entered matching")
	}
	if event, _ := observer.translate(key, 42, translate('a')); event.kind != inputText {
		t.Fatal("input did not resume after composition")
	}
	key.Modifiers = keyboard.ModifierCtrl
	if event, _ := observer.translate(key, 42, translate('a')); event.kind != inputInvalidate {
		t.Fatal("control shortcut entered matching")
	}
	key.Modifiers, key.NativeKeyCode = 0, 8
	if event, _ := observer.translate(key, 42, translate(0)); event.kind != inputBackspace {
		t.Fatal("backspace was not recognized")
	}
	key.Type = keyboard.EventTypeKeyUp
	if event, _ := observer.translate(key, 42, translate(0)); event.down {
		t.Fatal("key release was lost")
	}
}

// TestWindowsInjectedInputCancelsPendingExpansion covers a copy or automation arriving after a match.
func TestWindowsInjectedInputCancelsPendingExpansion(t *testing.T) {
	var observer windowsInputObserver
	var matcher engine
	matcher.reload([]Record{{ID: "test", Keyword: "a", Text: "expanded"}})
	key := keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyA, NativeKeyCode: 65}
	character := func(uint32, uint32) uint32 { return 'a' }
	down, _ := observer.translate(key, 42, character)
	matcher.onInput(down)
	key.Type = keyboard.EventTypeKeyUp
	up, _ := observer.translate(key, 42, character)
	work := matcher.onInput(up)
	if work == nil || !matcher.valid(*work) {
		t.Fatal("match did not produce pending work")
	}
	key.Type, key.NativeFlags, key.NativeExtraInfo = keyboard.EventTypeKeyDown, 0x10, windowsSnippetInputTag
	if event, ok := observer.translate(key, 42, character); ok {
		matcher.onInput(event)
	}
	if !matcher.valid(*work) {
		t.Fatal("own injected events must not cancel expansion")
	}
	key.NativeExtraInfo = 0
	if event, ok := observer.translate(key, 42, character); ok {
		matcher.onInput(event)
	}
	if matcher.valid(*work) {
		t.Fatal("another process's injected event must cancel expansion")
	}
}

// TestWindowsUnsafeTargetClearsMatching ensures unverified controls never retain typed characters.
func TestWindowsUnsafeTargetClearsMatching(t *testing.T) {
	var observer windowsInputObserver
	var e engine
	e.reload([]Record{{ID: "test", Keyword: "ab", Text: "expanded"}})
	key := keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyA, NativeKeyCode: 65}
	event, _ := observer.translate(key, 42, func(uint32, uint32) uint32 { return 'a' })
	e.onInput(event)
	event, _ = observer.translate(key, 0, func(uint32, uint32) uint32 { t.Fatal("unsafe key translated"); return 0 })
	e.onInput(event)
	if e.matcher.buffer != "" {
		t.Fatal("unsafe target retained matching buffer")
	}
}
