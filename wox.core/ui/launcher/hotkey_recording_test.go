package launcher

import (
	"context"
	"runtime"
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

func TestFallbackHotkeyStringRecordsPunctuationCombos(t *testing.T) {
	for _, tc := range []struct {
		key    woxui.Key
		hotkey string
	}{
		{woxui.Key(","), "ctrl+,"},
		{woxui.Key("."), "ctrl+."},
		{woxui.Key("/"), "ctrl+/"},
		{woxui.Key(";"), "ctrl+;"},
		{woxui.Key("'"), "ctrl+'"},
		{woxui.Key("["), "ctrl+["},
		{woxui.Key("]"), "ctrl+]"},
		{woxui.Key("\\"), "ctrl+\\"},
		{woxui.Key("-"), "ctrl+-"},
		{woxui.Key("="), "ctrl+="},
	} {
		if got := fallbackHotkeyString(woxui.KeyEvent{Key: tc.key, Down: true, Modifiers: woxui.KeyModifierControl}); got != tc.hotkey {
			t.Fatalf("%s = %q, want %q", tc.key, got, tc.hotkey)
		}
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

func (s *actionRecordingTestServices) StopHotkeyRecording(context.Context, string) error {
	return nil
}

func (s *actionRecordingTestServices) CheckHotkeyAvailability(context.Context, string, string) (contract.HotkeyAvailability, error) {
	return contract.HotkeyAvailability{Available: true}, nil
}

// newActionFormHotkeyTestApp builds a launcher app that can open action forms without a native loop.
func newActionFormHotkeyTestApp(services contract.Services) *App {
	deps := CommonDeps{}
	return &App{
		hotkeySettings: newHotkeySettingsController(deps),
		pluginSettings: newPluginSettingsController(deps),
		services:       services,
		lifecycleCtx:   context.Background(),
		window:         &woxui.Window{},
		editor:         woxui.NewTextEditor(""),
		translations:   map[string]string{},
		palette:        defaultPalette(),
		densityMetrics: launcherDensityMetricsFor(""),
		show:           showAppParams{WindowWidth: 800, MaxResultCount: 8},
	}
}

// TestOpenFormActionStartsHotkeyRecording starts capture as soon as a hotkey form opens.
func TestOpenFormActionStartsHotkeyRecording(t *testing.T) {
	services := &actionRecordingTestServices{kinds: make(chan []string, 1)}
	app := newActionFormHotkeyTestApp(services)
	app.openFormAction(queryResult{ID: "settings", QueryID: "q"}, resultAction{
		ID:   "__system_set_result_hotkey__",
		Form: []formDefinition{{Type: "hotkey", Value: formDefinitionValue{Key: "hotkey"}}},
	})
	recording := app.hotkeySettings.Recording()
	if recording == nil || recording.idPrefix != "action-form" || recording.fieldIndex != 0 {
		t.Fatalf("recording = %+v, want action-form field 0", recording)
	}
	select {
	case <-services.kinds:
	case <-time.After(time.Second):
		t.Fatal("opening a hotkey form did not start the recorder")
	}
	app.closeFormAction()
	if app.hotkeySettings.Recording() != nil {
		t.Fatal("closing the form must stop recording")
	}
}

// TestOpenFormActionSkipsRecordingForTextFields keeps alias and other text forms on the editor.
func TestOpenFormActionSkipsRecordingForTextFields(t *testing.T) {
	services := &actionRecordingTestServices{kinds: make(chan []string, 1)}
	app := newActionFormHotkeyTestApp(services)
	app.openFormAction(queryResult{ID: "settings", QueryID: "q"}, resultAction{
		ID:   "__system_set_result_alias__",
		Form: []formDefinition{{Type: "textbox", Value: formDefinitionValue{Key: "alias"}}},
	})
	if app.hotkeySettings.Recording() != nil {
		t.Fatal("alias form must not start the hotkey recorder")
	}
	select {
	case <-services.kinds:
		t.Fatal("textbox form started the recorder")
	case <-time.After(50 * time.Millisecond):
	}
}

// TestActionFormEscapeAndEnterStopRecordingOnce matches Settings: one
// Escape/Enter ends capture. The form panel Boundary must also miss cache so
// the recorder leaves "Recording..." on that same keypress.
func TestActionFormEscapeAndEnterStopRecordingOnce(t *testing.T) {
	for _, key := range []woxui.Key{woxui.KeyEscape, woxui.KeyEnter} {
		services := &actionRecordingTestServices{kinds: make(chan []string, 1)}
		app := newActionFormHotkeyTestApp(services)
		app.openFormAction(queryResult{ID: "settings", QueryID: "q"}, resultAction{
			ID:   "__system_set_result_hotkey__",
			Form: []formDefinition{{Type: "hotkey", Value: formDefinitionValue{Key: "hotkey"}}},
		})
		select {
		case <-services.kinds:
		case <-time.After(time.Second):
			t.Fatal("opening a hotkey form did not start the recorder")
		}
		formSnapshot := viewSnapshot{form: snapshotFormLocked(app.form), palette: app.palette, densityMetrics: app.densityMetrics}
		recordingSignature := app.launcherFormSectionSignature(formSnapshot, 400)
		if !app.onHotkeyRecordingKey(woxui.KeyEvent{Key: key, Down: true}) {
			t.Fatalf("%s should stop the action-form recorder", key)
		}
		if app.hotkeySettings.Recording() != nil {
			t.Fatalf("%s left the action-form recorder active", key)
		}
		if app.launcherFormSectionSignature(formSnapshot, 400) == recordingSignature {
			t.Fatalf("%s stopped recording but the form section cache would still show Recording...", key)
		}
		if app.onFormKey(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}) && app.hotkeySettings.Recording() != nil {
			t.Fatalf("a follow-up Enter after %s restarted recording", key)
		}
	}
}

type delayedAvailabilityServices struct {
	actionRecordingTestServices
	started chan struct{}
	release chan struct{}
}

func (s *delayedAvailabilityServices) CheckHotkeyAvailability(context.Context, string, string) (contract.HotkeyAvailability, error) {
	s.started <- struct{}{}
	<-s.release
	return contract.HotkeyAvailability{Available: false, ConflictType: "system"}, nil
}

func TestApplyRecordedHotkeyShowsCandidateBeforeAvailability(t *testing.T) {
	services := &delayedAvailabilityServices{
		actionRecordingTestServices: actionRecordingTestServices{kinds: make(chan []string, 1)},
		started:                     make(chan struct{}, 1),
		release:                     make(chan struct{}),
	}
	app := newActionFormHotkeyTestApp(services)
	app.openFormAction(queryResult{ID: "settings", QueryID: "q"}, resultAction{
		ID:   "__system_set_result_hotkey__",
		Form: []formDefinition{{Type: "hotkey", Value: formDefinitionValue{Key: "hotkey"}}},
	})
	select {
	case <-services.kinds:
	case <-time.After(time.Second):
		t.Fatal("opening a hotkey form did not start the recorder")
	}
	if err := app.applyRecordedHotkey(recordedHotkeyPayload{Hotkey: "ctrl+shift+p", Kind: "normalCombo"}); err != nil {
		t.Fatal(err)
	}
	state := app.hotkeySettings.Recording()
	if state == nil || state.display != "ctrl+shift+p" || state.statusError || !state.checking {
		t.Fatalf("recording display = %+v, want keycaps before the availability probe finishes", state)
	}
	select {
	case <-services.started:
	case <-time.After(time.Second):
		t.Fatal("availability check did not start")
	}
	close(services.release)
}

type actionFormSubmitServices struct {
	actionRecordingTestServices
	submitted chan map[string]string
}

func (s *actionFormSubmitServices) SubmitFormAction(_ context.Context, _, _, _, _ string, values map[string]string) error {
	s.submitted <- values
	return nil
}

// TestActionFormPrimaryEnterSavesWhileRecording matches the Save (Ctrl/Cmd+Enter)
// hint: the recorder yields that chord so the form handler can submit the last shown combo.
func TestActionFormPrimaryEnterSavesWhileRecording(t *testing.T) {
	services := &actionFormSubmitServices{
		actionRecordingTestServices: actionRecordingTestServices{kinds: make(chan []string, 1)},
		submitted:                   make(chan map[string]string, 1),
	}
	app := newActionFormHotkeyTestApp(services)
	app.openFormAction(queryResult{ID: "settings", QueryID: "q"}, resultAction{
		ID:   "__system_set_result_hotkey__",
		Form: []formDefinition{{Type: "hotkey", Value: formDefinitionValue{Key: "hotkey"}}},
	})
	select {
	case <-services.kinds:
	case <-time.After(time.Second):
		t.Fatal("opening a hotkey form did not start the recorder")
	}
	recording := app.hotkeySettings.Recording()
	recording.display = "ctrl+shift+,"
	recording.checking = true
	saveHotkey := "ctrl+enter"
	if runtime.GOOS == "darwin" {
		saveHotkey = "command+enter"
	}
	if err := app.applyRecordedHotkey(recordedHotkeyPayload{Hotkey: saveHotkey, Kind: "normalCombo"}); err != nil {
		t.Fatal(err)
	}
	if app.form != nil {
		t.Fatal("Ctrl/Cmd+Enter left the action form open")
	}
	if app.hotkeySettings.Recording() != nil {
		t.Fatal("Ctrl/Cmd+Enter left the action-form recorder active")
	}
	select {
	case values := <-services.submitted:
		if values["hotkey"] != "ctrl+shift+," {
			t.Fatalf("submitted hotkey = %q, want the last shown combo", values["hotkey"])
		}
	case <-time.After(time.Second):
		t.Fatal("Ctrl/Cmd+Enter did not submit the action form")
	}
}

func TestRecordedHotkeyWithoutParentHookStaysACandidate(t *testing.T) {
	fields := newFormFieldsState([]formDefinition{{Type: "hotkey", Value: formDefinitionValue{Key: "MainHotkey"}}}, nil, true)
	controller := newHotkeySettingsController(CommonDeps{})
	controller.SetForm(&fields)
	controller.SetRecording(&hotkeyRecordingState{
		target: &fields, fieldIndex: 0, idPrefix: "hotkey-settings", allowed: map[string]bool{"normalCombo": true},
		diagnosticCtx: context.Background(),
	})
	app := &App{
		hotkeySettings: controller, pluginSettings: newPluginSettingsController(CommonDeps{}),
		settingsOpen: true, settingTab: "hotkey", services: &actionRecordingTestServices{}, lifecycleCtx: context.Background(),
	}
	if err := app.applyRecordedHotkey(recordedHotkeyPayload{Hotkey: "ctrl+enter", Kind: "normalCombo"}); err != nil {
		t.Fatal(err)
	}
	state := app.hotkeySettings.Recording()
	if state == nil || state.display != "ctrl+enter" {
		t.Fatalf("settings recording display = %+v, want ctrl+enter kept as a candidate", state)
	}
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
