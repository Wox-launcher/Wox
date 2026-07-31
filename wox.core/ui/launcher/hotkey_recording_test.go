package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestCanonicalRecordedHotkeyPrefixesHoldModifier(t *testing.T) {
	if got := canonicalRecordedHotkey(recordedHotkeyPayload{Hotkey: "cmd", Kind: "holdModifier"}); got != "hold:cmd" {
		t.Fatalf("canonical hold hotkey = %q, want hold:cmd", got)
	}
	if got := canonicalRecordedHotkey(recordedHotkeyPayload{Hotkey: "hold:shift", Kind: "holdModifier"}); got != "hold:shift" {
		t.Fatalf("canonical existing hold hotkey = %q, want hold:shift", got)
	}
}

func TestModifierOnlyHotkeysSkipAvailabilityCheck(t *testing.T) {
	for _, kind := range []string{"pressModifier", "holdModifier"} {
		if !hotkeyKindSkipsAvailability(kind) {
			t.Fatalf("%s should be accepted without an availability check", kind)
		}
	}
	for _, kind := range []string{"normalCombo", "doubleModifier", "capsLockCombo"} {
		if hotkeyKindSkipsAvailability(kind) {
			t.Fatalf("%s should still use the availability check", kind)
		}
	}
}

func TestHotkeyRecordingPresentationKeepsConflictCandidate(t *testing.T) {
	controller := newHotkeySettingsController(CommonDeps{})
	controller.SetRecording(&hotkeyRecordingState{
		idPrefix: "plugin-settings", fieldIndex: 2, display: "cmd+a", status: "conflict", statusError: true,
	})
	app := &App{hotkeySettings: controller}

	presentation := app.hotkeyRecordingFieldStatus("plugin-settings", 2)
	if !presentation.Active || presentation.Value != "cmd+a" || presentation.Status != "conflict" || !presentation.Error {
		t.Fatalf("recording presentation = %+v", presentation)
	}
}

func TestHotkeyRecordingFocusKeysMatchFlutter(t *testing.T) {
	if !hotkeyRecordingMovesFocus(woxui.KeyEvent{Key: woxui.KeyTab}) {
		t.Fatal("Tab should move focus from the recorder")
	}
	if !hotkeyRecordingMovesFocus(woxui.KeyEvent{Key: woxui.KeyTab, Modifiers: woxui.KeyModifierShift}) {
		t.Fatal("Shift+Tab should move focus backward from the recorder")
	}
	if hotkeyRecordingMovesFocus(woxui.KeyEvent{Key: woxui.KeyTab, Modifiers: woxui.KeyModifierControl}) {
		t.Fatal("Ctrl+Tab should remain available as a shortcut candidate")
	}
	if !hotkeyRecordingMovesFocus(woxui.KeyEvent{Key: woxui.KeyEnter}) {
		t.Fatal("Enter should move focus from the recorder")
	}
	if hotkeyRecordingMovesFocus(woxui.KeyEvent{Key: woxui.KeyEnter, Modifiers: woxui.KeyModifierShift}) {
		t.Fatal("Shift+Enter should remain available as a shortcut candidate")
	}
}

func TestEscapeKeepsHotkeyRecorderFocused(t *testing.T) {
	controller := newHotkeySettingsController(CommonDeps{})
	state := &hotkeyRecordingState{}
	controller.SetRecording(state)
	app := &App{hotkeySettings: controller}

	if !app.onHotkeyRecordingKey(woxui.KeyEvent{Key: woxui.KeyEscape}) {
		t.Fatal("Escape should be consumed while recording")
	}
	if controller.Recording() != state {
		t.Fatal("Escape should not stop the recorder")
	}
}
