package hotkey

import (
	"errors"
	"testing"
	"time"
	"wox/util/keyboard"
)

func TestRecordingSessionAllowsOnlyNormalComboFallbackWhenRawUnavailable(t *testing.T) {
	restore := replaceRawKeyListenerForTest(t, func(handler keyboard.RawKeyHandler) (keyboard.RawKeySubscription, error) {
		return nil, errors.New("raw listener unavailable")
	})
	defer restore()

	recorded := []recordedHotkey{}
	manager := newHotkeyRecordingSessionManager()
	capability, err := manager.Start(recordingSessionOptions{
		allowedKinds: []hotkeyKind{hotkeyKindNormalCombo, hotkeyKindPressModifier},
		onRecorded: func(result recordedHotkey) {
			recorded = append(recorded, result)
		},
	})
	if err != nil {
		t.Fatalf("start recording session: %v", err)
	}
	defer manager.Stop()

	if capability.RawRecorderAvailable {
		t.Fatalf("expected raw recorder to be unavailable")
	}
	if len(capability.FallbackAllowedKinds) != 1 || capability.FallbackAllowedKinds[0] != hotkeyKindNormalCombo {
		t.Fatalf("expected only normalCombo fallback, got %v", capability.FallbackAllowedKinds)
	}

	if err := manager.SubmitFallbackCandidate("ctrl+space"); err != nil {
		t.Fatalf("submit normal fallback: %v", err)
	}
	if len(recorded) != 1 || recorded[0].Hotkey != "ctrl+space" || recorded[0].Kind != hotkeyKindNormalCombo {
		t.Fatalf("expected normal fallback recording, got %+v", recorded)
	}

	if err := manager.SubmitFallbackCandidate("left_alt"); err == nil {
		t.Fatalf("expected modifier-only fallback candidate to be rejected")
	}
}

func TestStartRecordingSessionRejectsEmptyAllowedKinds(t *testing.T) {
	if _, err := StartRecordingSession(nil, nil); err == nil {
		t.Fatalf("expected empty allowedKinds to be rejected")
	}
}

func TestRecordingSessionCapturesRawModifierPressWhenAvailable(t *testing.T) {
	var rawHandler keyboard.RawKeyHandler
	restore := replaceRawKeyListenerForTest(t, func(handler keyboard.RawKeyHandler) (keyboard.RawKeySubscription, error) {
		rawHandler = handler
		return noopRawKeySubscription{}, nil
	})
	defer restore()

	recorded := []recordedHotkey{}
	manager := newHotkeyRecordingSessionManager()
	capability, err := manager.Start(recordingSessionOptions{
		allowedKinds: []hotkeyKind{hotkeyKindPressModifier},
		onRecorded: func(result recordedHotkey) {
			recorded = append(recorded, result)
		},
	})
	if err != nil {
		t.Fatalf("start recording session: %v", err)
	}
	defer manager.Stop()

	if !capability.RawRecorderAvailable {
		t.Fatalf("expected raw recorder to be available")
	}
	if rawHandler == nil {
		t.Fatalf("expected raw handler to be installed")
	}

	rawHandler(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftAlt))
	rawHandler(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftAlt))

	if len(recorded) != 1 || recorded[0].Hotkey != "left_alt" || recorded[0].Kind != hotkeyKindPressModifier {
		t.Fatalf("expected raw press modifier recording, got %+v", recorded)
	}
}

func TestRecordingSessionCapturesHoldWithoutTrailingPressWhenBothModifierKindsAllowed(t *testing.T) {
	var rawHandler keyboard.RawKeyHandler
	restore := replaceRawKeyListenerForTest(t, func(handler keyboard.RawKeyHandler) (keyboard.RawKeySubscription, error) {
		rawHandler = handler
		return noopRawKeySubscription{}, nil
	})
	defer restore()

	recorded := make(chan recordedHotkey, 4)
	manager := newHotkeyRecordingSessionManager()
	if _, err := manager.Start(recordingSessionOptions{
		allowedKinds: []hotkeyKind{hotkeyKindHoldModifier, hotkeyKindPressModifier},
		onRecorded: func(result recordedHotkey) {
			recorded <- result
		},
	}); err != nil {
		t.Fatalf("start recording session: %v", err)
	}
	defer manager.Stop()

	if rawHandler == nil {
		t.Fatalf("expected raw handler to be installed")
	}

	rawHandler(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftAlt))
	assertRecordedHotkey(t, recorded, recordedHotkey{Hotkey: "left_alt", Kind: hotkeyKindHoldModifier})
	rawHandler(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftAlt))
	assertNoRecordedHotkey(t, recorded, "trailing press after hold")
}

func replaceRawKeyListenerForTest(t *testing.T, replacement func(keyboard.RawKeyHandler) (keyboard.RawKeySubscription, error)) func() {
	t.Helper()

	previous := addRawKeyListener
	addRawKeyListener = replacement
	return func() {
		addRawKeyListener = previous
	}
}

type noopRawKeySubscription struct{}

func (noopRawKeySubscription) Close() error {
	return nil
}

func assertRecordedHotkey(t *testing.T, ch <-chan recordedHotkey, expected recordedHotkey) {
	t.Helper()

	select {
	case actual := <-ch:
		if actual != expected {
			t.Fatalf("expected recorded hotkey %+v, got %+v", expected, actual)
		}
	case <-time.After(recordingModifierChordDebounce + 800*time.Millisecond):
		t.Fatalf("expected recorded hotkey %+v", expected)
	}
}

func assertNoRecordedHotkey(t *testing.T, ch <-chan recordedHotkey, label string) {
	t.Helper()

	select {
	case actual := <-ch:
		t.Fatalf("did not expect %s, got %+v", label, actual)
	case <-time.After(recordingModifierChordDebounce + 80*time.Millisecond):
	}
}
