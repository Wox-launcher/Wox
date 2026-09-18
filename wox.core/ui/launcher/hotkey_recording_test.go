package launcher

import (
	"context"
	"testing"
	"time"
	"wox/ui/contract"

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
	if !hotkeyRecordingStops(woxui.KeyEvent{Key: woxui.KeyEscape}) {
		t.Fatal("Escape should stop the recorder")
	}
	if !hotkeyRecordingStops(woxui.KeyEvent{Key: woxui.KeyTab}) {
		t.Fatal("Tab should stop the recorder")
	}
	if !hotkeyRecordingStops(woxui.KeyEvent{Key: woxui.KeyTab, Modifiers: woxui.KeyModifierShift}) {
		t.Fatal("Shift+Tab should stop the recorder")
	}
	if hotkeyRecordingStops(woxui.KeyEvent{Key: woxui.KeyTab, Modifiers: woxui.KeyModifierControl}) {
		t.Fatal("Ctrl+Tab should remain available as a shortcut candidate")
	}
	if !hotkeyRecordingStops(woxui.KeyEvent{Key: woxui.KeyEnter}) {
		t.Fatal("Enter should stop the recorder")
	}
	if hotkeyRecordingStops(woxui.KeyEvent{Key: woxui.KeyEnter, Modifiers: woxui.KeyModifierShift}) {
		t.Fatal("Shift+Enter should remain available as a shortcut candidate")
	}
}

func TestFallbackHotkeyStringAllowsStandaloneFunctionKeys(t *testing.T) {
	if got := fallbackHotkeyString(woxui.KeyEvent{Key: woxui.KeySpace, Down: true, Modifiers: woxui.KeyModifierAlt}); got != "alt+space" {
		t.Fatalf("Alt+Space = %q", got)
	}
	if got := fallbackHotkeyString(woxui.KeyEvent{Key: woxui.Key("f12"), Down: true}); got != "f12" {
		t.Fatalf("standalone F12 = %q, want f12", got)
	}
	if got := fallbackHotkeyString(woxui.KeyEvent{Key: woxui.Key("a"), Down: true}); got != "" {
		t.Fatalf("standalone letter = %q, want empty", got)
	}
}

// localHotkeyTestServices observes candidates without starting native windows.
type localHotkeyTestServices struct {
	contract.Services
	candidates chan string
}

func (s *localHotkeyTestServices) SubmitHotkeyRecordingCandidate(_ context.Context, _ string, hotkey string) error {
	s.candidates <- hotkey
	return nil
}

// TestHotkeyLocalFallbackWithRawAvailable reproduces the UI state of a partially
// functioning event tap and ensures the local candidate still reaches core.
func TestHotkeyLocalFallbackWithRawAvailable(t *testing.T) {
	services := &localHotkeyTestServices{candidates: make(chan string, 1)}
	controller := newHotkeySettingsController(CommonDeps{})
	controller.SetRecording(&hotkeyRecordingState{
		ready: true, raw: true, fallback: true, diagnosticCtx: context.Background(),
	})
	app := &App{hotkeySettings: controller, services: services, lifecycleCtx: context.Background()}
	if !app.onHotkeyRecordingKey(woxui.KeyEvent{Key: woxui.Key("a"), Down: true, Modifiers: woxui.KeyModifierControl}) {
		t.Fatal("recording did not consume the local key")
	}
	select {
	case candidate := <-services.candidates:
		if candidate != "ctrl+a" {
			t.Fatalf("unexpected candidate: %s", candidate)
		}
	case <-time.After(time.Second):
		t.Fatal("raw availability disabled local fallback")
	}
}

// TestFallbackHotkeyStringIgnoresPureModifiers keeps dictation modifier input
// out of the normal-combo path while preserving the following ordinary key.
func TestFallbackHotkeyStringIgnoresPureModifiers(t *testing.T) {
	modifiers := woxui.KeyModifierControl | woxui.KeyModifierShift | woxui.KeyModifierAlt | woxui.KeyModifierMeta
	for _, key := range []woxui.Key{woxui.KeyAlt, woxui.KeyMeta, "shift", "ctrl", "control"} {
		if got := fallbackHotkeyString(woxui.KeyEvent{Key: key, Down: true, Modifiers: modifiers}); got != "" {
			t.Fatalf("modifier %s produced candidate %q", key, got)
		}
	}
	if got := fallbackHotkeyString(woxui.KeyEvent{Key: woxui.Key("c"), Down: true, Modifiers: woxui.KeyModifierControl | woxui.KeyModifierShift}); got != "ctrl+shift+c" {
		t.Fatalf("normal combination changed: %q", got)
	}
}

// actionRecordingTestServices captures the kinds requested from the native recorder.
type actionRecordingTestServices struct {
	contract.Services
	kinds chan []string
}

func (s *actionRecordingTestServices) StartHotkeyRecording(_ context.Context, _ string, _ string, kinds []string) (contract.HotkeyRecordingCapability, error) {
	s.kinds <- kinds
	return contract.HotkeyRecordingCapability{}, nil
}

func TestActionHotkeyRecordingOnlyAllowsNormalCombos(t *testing.T) {
	services := &actionRecordingTestServices{kinds: make(chan []string, 1)}
	app := newApp(false, services, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	app.settingsOpen = true
	app.settingTab = "hotkey"
	form := newHotkeySettingsForm(settingsData{})
	app.hotkeySettings.SetForm(&form)
	for index, definition := range form.definitions {
		if definition.Value.Key == "ActionPanelHotkey" {
			app.recordHotkeySettingsField(index)
			break
		}
	}
	select {
	case kinds := <-services.kinds:
		if len(kinds) != 1 || kinds[0] != "normalCombo" {
			t.Fatalf("action shortcut recording kinds = %v", kinds)
		}
	case <-time.After(time.Second):
		t.Fatal("recorder did not start")
	}
}
