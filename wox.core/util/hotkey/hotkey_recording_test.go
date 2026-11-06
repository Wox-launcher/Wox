package hotkey

import (
	"errors"
	"testing"
	"time"
	"wox/util/keyboard"
)

// TestRecordingDiagnosticsCountsPastDetailLimit checks that diagnostics preserve rejection evidence without changing recording.
func TestRecordingDiagnosticsCountsPastDetailLimit(t *testing.T) {
	var candidates []recordedHotkey
	state := newRecordingRawState(hotkeyKindSet([]hotkeyKind{hotkeyKindNormalCombo}), func(candidate recordedHotkey) {
		candidates = append(candidates, candidate)
	})
	for i := 0; i < 45; i++ {
		if state.HandleEvent(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyA}) {
			t.Fatal("unmodified letter was consumed")
		}
	}
	state.HandleEvent(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyLeftCtrl})
	state.HandleEvent(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyUnknown})
	if !state.HandleEvent(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyA}) {
		t.Fatal("normal combo was not consumed")
	}
	state.Close()
	if state.eventCount != 48 || state.candidateCount != 1 || state.eventReasons["no_tracked_modifier"] != 45 || state.eventReasons["unsupported_key"] != 1 || state.eventReasons["normal_combo_candidate"] != 1 {
		t.Fatalf("unexpected diagnostics: events=%d candidates=%d reasons=%v", state.eventCount, state.candidateCount, state.eventReasons)
	}
	if len(candidates) != 1 || candidates[0].Hotkey != "ctrl+a" {
		t.Fatalf("recording changed: %+v", candidates)
	}
	if state.diagnosticCtx.Err() == nil {
		t.Fatal("closing recorder did not cancel diagnostics")
	}
}

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

func TestRecordingSessionCapturesModifiedFunctionKeys(t *testing.T) {
	var rawHandler keyboard.RawKeyHandler
	restore := replaceRawKeyListenerForTest(t, func(handler keyboard.RawKeyHandler) (keyboard.RawKeySubscription, error) {
		rawHandler = handler
		return noopRawKeySubscription{}, nil
	})
	defer restore()

	recorded := make(chan recordedHotkey, 1)
	manager := newHotkeyRecordingSessionManager()
	if _, err := manager.Start(recordingSessionOptions{
		allowedKinds: []hotkeyKind{hotkeyKindNormalCombo},
		onRecorded:   func(result recordedHotkey) { recorded <- result },
	}); err != nil {
		t.Fatalf("start recording session: %v", err)
	}
	defer manager.Stop()

	rawHandler(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftCtrl))
	if consumed := rawHandler(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyF12)); !consumed {
		t.Fatal("recorded Ctrl+F12 should be consumed")
	}
	assertRecordedHotkey(t, recorded, recordedHotkey{Hotkey: "ctrl+f12", Kind: hotkeyKindNormalCombo})
}

func TestRecordingSessionCapturesPunctuationCombos(t *testing.T) {
	var rawHandler keyboard.RawKeyHandler
	restore := replaceRawKeyListenerForTest(t, func(handler keyboard.RawKeyHandler) (keyboard.RawKeySubscription, error) {
		rawHandler = handler
		return noopRawKeySubscription{}, nil
	})
	defer restore()

	recorded := make(chan recordedHotkey, 1)
	manager := newHotkeyRecordingSessionManager()
	if _, err := manager.Start(recordingSessionOptions{
		allowedKinds: []hotkeyKind{hotkeyKindNormalCombo},
		onRecorded:   func(result recordedHotkey) { recorded <- result },
	}); err != nil {
		t.Fatalf("start recording session: %v", err)
	}
	defer manager.Stop()

	rawHandler(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftCtrl))
	for _, tc := range []struct {
		key    keyboard.Key
		hotkey string
	}{
		{keyboard.KeyComma, "ctrl+,"},
		{keyboard.KeyPeriod, "ctrl+."},
		{keyboard.KeySlash, "ctrl+/"},
		{keyboard.KeySemicolon, "ctrl+;"},
		{keyboard.KeyApostrophe, "ctrl+'"},
		{keyboard.KeyLeftBracket, "ctrl+["},
		{keyboard.KeyRightBracket, "ctrl+]"},
		{keyboard.KeyBackslash, "ctrl+\\"},
		{keyboard.KeyMinus, "ctrl+-"},
		{keyboard.KeyEqual, "ctrl+="},
	} {
		if consumed := rawHandler(rawModifierEvent(keyboard.EventTypeKeyDown, tc.key)); !consumed {
			t.Fatalf("recorded %s should be consumed", tc.hotkey)
		}
		assertRecordedHotkey(t, recorded, recordedHotkey{Hotkey: tc.hotkey, Kind: hotkeyKindNormalCombo})
	}
}

func TestRecordingSessionCapturesStandaloneFunctionKeys(t *testing.T) {
	var rawHandler keyboard.RawKeyHandler
	restore := replaceRawKeyListenerForTest(t, func(handler keyboard.RawKeyHandler) (keyboard.RawKeySubscription, error) {
		rawHandler = handler
		return noopRawKeySubscription{}, nil
	})
	defer restore()

	recorded := make(chan recordedHotkey, 1)
	manager := newHotkeyRecordingSessionManager()
	if _, err := manager.Start(recordingSessionOptions{
		allowedKinds: []hotkeyKind{hotkeyKindNormalCombo},
		onRecorded:   func(result recordedHotkey) { recorded <- result },
	}); err != nil {
		t.Fatalf("start recording session: %v", err)
	}
	defer manager.Stop()

	if consumed := rawHandler(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyF12)); !consumed {
		t.Fatal("recorded standalone F12 should be consumed")
	}
	assertRecordedHotkey(t, recorded, recordedHotkey{Hotkey: "f12", Kind: hotkeyKindNormalCombo})
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

// TestRecordingModifierPressBelowHoldThreshold keeps a deliberate short press as a press.
func TestRecordingModifierPressBelowHoldThreshold(t *testing.T) {
	recorded := make(chan recordedHotkey, 2)
	state := newRecordingRawState(hotkeyKindSet([]hotkeyKind{hotkeyKindHoldModifier, hotkeyKindPressModifier}), func(result recordedHotkey) {
		recorded <- result
	})
	defer state.Close()

	state.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftAlt))
	time.Sleep(150 * time.Millisecond)
	state.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyLeftAlt))
	assertRecordedHotkey(t, recorded, recordedHotkey{Hotkey: "left_alt", Kind: hotkeyKindPressModifier})
	assertNoRecordedHotkey(t, recorded, "hold after a short press")
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
	case <-time.After(holdPressDelay + 800*time.Millisecond):
		t.Fatalf("expected recorded hotkey %+v", expected)
	}
}

func assertNoRecordedHotkey(t *testing.T, ch <-chan recordedHotkey, label string) {
	t.Helper()

	select {
	case actual := <-ch:
		t.Fatalf("did not expect %s, got %+v", label, actual)
	case <-time.After(holdPressDelay + 80*time.Millisecond):
	}
}

// TestRecordingSessionSupplementsModifierOnlyDelivery covers a successful raw
// subscription that never delivers the ordinary key of a normal combination.
func TestRecordingSessionSupplementsModifierOnlyDelivery(t *testing.T) {
	var rawHandler keyboard.RawKeyHandler
	restore := replaceRawKeyListenerForTest(t, func(handler keyboard.RawKeyHandler) (keyboard.RawKeySubscription, error) {
		rawHandler = handler
		return noopRawKeySubscription{}, nil
	})
	defer restore()
	var recorded []recordedHotkey
	manager := newHotkeyRecordingSessionManager()
	capability, err := manager.Start(recordingSessionOptions{
		allowedKinds: []hotkeyKind{hotkeyKindNormalCombo, hotkeyKindDoubleModifier},
		onRecorded:   func(result recordedHotkey) { recorded = append(recorded, result) },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Stop()
	if !capability.RawRecorderAvailable || len(capability.FallbackAllowedKinds) != 1 || capability.FallbackAllowedKinds[0] != hotkeyKindNormalCombo {
		t.Fatalf("partial raw delivery must retain normal fallback: %+v", capability)
	}
	rawHandler(keyboard.RawKeyEvent{Type: keyboard.EventTypeKeyDown, Key: keyboard.KeyLeftCtrl})
	if err := manager.SubmitFallbackCandidate("ctrl+a"); err != nil {
		t.Fatal(err)
	}
	if len(recorded) != 1 || recorded[0].Hotkey != "ctrl+a" || recorded[0].Kind != hotkeyKindNormalCombo {
		t.Fatalf("local combo was not recorded: %+v", recorded)
	}
	if err := manager.SubmitFallbackCandidate("ctrl+ctrl"); err == nil {
		t.Fatal("local fallback must not record double modifiers")
	}
}

func TestRecordingInsertAloneAndWithModifier(t *testing.T) {
	for _, tc := range []struct {
		name     string
		modifier keyboard.Key
		want     string
	}{
		{name: "standalone", want: "insert"},
		{name: "control", modifier: keyboard.KeyLeftCtrl, want: "ctrl+insert"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var recorded []recordedHotkey
			state := newRecordingRawState(hotkeyKindSet([]hotkeyKind{hotkeyKindNormalCombo}), func(candidate recordedHotkey) {
				recorded = append(recorded, candidate)
			})
			defer state.Close()
			if tc.modifier != keyboard.KeyUnknown {
				state.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, tc.modifier))
			}
			if !state.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyInsert)) {
				t.Fatal("Insert was not consumed")
			}
			if len(recorded) != 1 || recorded[0].Hotkey != tc.want || recorded[0].Kind != hotkeyKindNormalCombo {
				t.Fatalf("recorded = %+v, want %s", recorded, tc.want)
			}
		})
	}
	if got := capsLockComboToHotkeyString(keyboard.KeyInsert); got != "capslock+insert" {
		t.Fatalf("CapsLock+Insert = %q", got)
	}
}

func TestRecordingDeleteAloneAndWithModifier(t *testing.T) {
	for _, tc := range []struct {
		name     string
		modifier keyboard.Key
		want     string
	}{
		{name: "standalone", want: "delete"},
		{name: "control", modifier: keyboard.KeyLeftCtrl, want: "ctrl+delete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var recorded []recordedHotkey
			state := newRecordingRawState(hotkeyKindSet([]hotkeyKind{hotkeyKindNormalCombo}), func(candidate recordedHotkey) {
				recorded = append(recorded, candidate)
			})
			defer state.Close()
			if tc.modifier != keyboard.KeyUnknown {
				state.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, tc.modifier))
			}
			if !state.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyDelete)) {
				t.Fatal("Delete was not consumed")
			}
			if len(recorded) != 1 || recorded[0].Hotkey != tc.want || recorded[0].Kind != hotkeyKindNormalCombo {
				t.Fatalf("recorded = %+v, want %s", recorded, tc.want)
			}
		})
	}
}

// TestRecordingStandaloneHoldDistinguishesTapAndHold exercises the recorder's shared threshold.
func TestRecordingStandaloneHoldDistinguishesTapAndHold(t *testing.T) {
	if !keyboard.SupportsStandaloneHold(keyboard.KeyDelete) {
		t.Skip("standalone hold interception unavailable")
	}
	recorded := make(chan recordedHotkey, 3)
	state := newRecordingRawState(hotkeyKindSet([]hotkeyKind{hotkeyKindNormalCombo, hotkeyKindHoldKey}), func(candidate recordedHotkey) { recorded <- candidate })
	defer state.Close()
	down := rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyDelete)
	up := rawModifierEvent(keyboard.EventTypeKeyUp, keyboard.KeyDelete)
	if !state.HandleEvent(down) || !state.HandleEvent(up) {
		t.Fatal("tap was not consumed")
	}
	assertRecordedHotkey(t, recorded, recordedHotkey{Hotkey: "delete", Kind: hotkeyKindNormalCombo})
	if !state.HandleEvent(down) || !state.HandleEvent(down) {
		t.Fatal("hold or repeat was not consumed")
	}
	assertRecordedHotkey(t, recorded, recordedHotkey{Hotkey: "delete", Kind: hotkeyKindHoldKey})
	state.HandleEvent(up)
	assertNoRecordedHotkey(t, recorded, "tap after hold")
}

// TestRecordingStandaloneHoldKeepsModifiedCombos checks normal shortcuts while hold is allowed.
func TestRecordingStandaloneHoldKeepsModifiedCombos(t *testing.T) {
	if !keyboard.SupportsStandaloneHold(keyboard.KeyDelete) {
		t.Skip("standalone hold interception unavailable")
	}
	recorded := make(chan recordedHotkey, 1)
	state := newRecordingRawState(hotkeyKindSet([]hotkeyKind{hotkeyKindNormalCombo, hotkeyKindHoldKey}), func(candidate recordedHotkey) { recorded <- candidate })
	defer state.Close()
	state.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyLeftCtrl))
	state.HandleEvent(rawModifierEvent(keyboard.EventTypeKeyDown, keyboard.KeyDelete))
	assertRecordedHotkey(t, recorded, recordedHotkey{Hotkey: "ctrl+delete", Kind: hotkeyKindNormalCombo})
}
