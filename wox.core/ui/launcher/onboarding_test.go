package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"testing"

	"wox/common"
	"wox/plugin/thirdparty/migrate"
	"wox/resource"
	"wox/ui/contract"
	"wox/util/keyboard"
)

type onboardingHotkeyTestServices struct {
	contract.Services
	err error
}

func (s *onboardingHotkeyTestServices) UpdateGeneralSetting(context.Context, string, string, string) error {
	return s.err
}

func TestOnboardingRetainsHotkeySaveFailureAfterRecorderCloses(t *testing.T) {
	services := &onboardingHotkeyTestServices{err: fmt.Errorf("portal: %w", keyboard.ErrGlobalHotkeysUnavailable)}
	app := &App{
		services: services, onboardingOpen: true,
		generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
		translations:    map[string]string{"ui_hotkey_registration_unsupported": "Registration unavailable"},
	}
	form := newFormFieldsState(nil, map[string]string{"MainHotkey": "Alt+K"}, true)
	state := &hotkeyRecordingState{diagnosticCtx: context.Background(), target: &form, persistKey: "MainHotkey"}
	app.saveRecordedHotkeySetting(state, "MainHotkey", "Alt+K", "Ctrl+Space")
	if app.onboardingError != "Registration unavailable" || form.values["MainHotkey"] != "Ctrl+Space" {
		t.Fatalf("failed save: error=%q hotkey=%q", app.onboardingError, form.values["MainHotkey"])
	}
	services.err = nil
	app.saveRecordedHotkeySetting(state, "MainHotkey", "Alt+K", "Ctrl+Space")
	if app.onboardingError != "" || app.generalSettings.Data().MainHotkey != "Alt+K" {
		t.Fatalf("successful retry: error=%q hotkey=%q", app.onboardingError, app.generalSettings.Data().MainHotkey)
	}
}

func TestOnboardingMigrationStepFollowsWelcome(t *testing.T) {
	plain := (&App{}).onboardingSteps()
	for _, step := range plain {
		if step.ID == "migrate" {
			t.Fatal("migration step appears without a detected launcher")
		}
	}
	if plain[len(plain)-1].ID != "finish" {
		t.Fatalf("last step = %q", plain[len(plain)-1].ID)
	}
	app := &App{onboardingMigration: onboardingMigrationState{installations: []migrate.Installation{
		onboardingMigrationStub{id: "flow", name: "Flow Launcher"},
		onboardingMigrationStub{id: "other", name: "Other"},
	}}}
	steps := app.onboardingSteps()
	wantIndex := 1
	if runtime.GOOS == "darwin" {
		wantIndex = 2
		if steps[1].ID != "permissions" {
			t.Fatalf("steps %#v", steps)
		}
	}
	if steps[0].ID != "welcome" || steps[wantIndex].ID != "migrate" || steps[wantIndex+1].ID != "mainHotkey" {
		t.Fatalf("steps start %#v", []string{steps[0].ID, steps[wantIndex].ID, steps[wantIndex+1].ID})
	}
	app.onboardingStep = wantIndex
	app.onboardingMigration.selectedID = "flow"
	app.selectOnboardingStep(wantIndex - 1)
	if app.onboardingStep != wantIndex || !app.onboardingMigration.choosing || app.onboardingMigration.selectedID != "" {
		t.Fatalf("back from items step=%d choosing=%v selected=%q", app.onboardingStep, app.onboardingMigration.choosing, app.onboardingMigration.selectedID)
	}
}

func TestSingleDetectedLauncherSkipsTheChooser(t *testing.T) {
	one := newOnboardingMigrationState([]migrate.Installation{onboardingMigrationStub{id: "flow", name: "Flow Launcher"}})
	if one.choosing || one.selectedID != "flow" {
		t.Fatalf("one launcher = choosing %v selected %q", one.choosing, one.selectedID)
	}
	many := newOnboardingMigrationState([]migrate.Installation{
		onboardingMigrationStub{id: "flow"}, onboardingMigrationStub{id: "other"},
	})
	if !many.choosing || many.selectedID != "" {
		t.Fatalf("many launchers = choosing %v selected %q", many.choosing, many.selectedID)
	}
}

func TestNormalizeLauncherHotkeyAcceptsFlowSpacing(t *testing.T) {
	got, ok := normalizeLauncherHotkey("Alt + Space")
	if !ok || got != "Alt+Space" {
		t.Fatalf("hotkey = %q ok=%v", got, ok)
	}
	if _, ok := normalizeLauncherHotkey("not a hotkey"); ok {
		t.Fatal("accepted an invalid hotkey")
	}
}

func TestMigrationHotkeyMessageDoesNotNameTheOccupant(t *testing.T) {
	taken := fmt.Errorf("failed to register hotkey (err=1409)")
	if got := migrationHotkeyPageMessage(taken); got != "i18n:onboarding_migrate_hotkey_next" {
		t.Fatalf("occupied shortcut message = %q", got)
	}
	if got := migrationApplyError(taken); got != migrationHotkeyTaken {
		t.Fatalf("occupied shortcut row = %q", got)
	}
	generic := fmt.Errorf("failed to register main hotkey: Alt+Space")
	if got := migrationHotkeyPageMessage(generic); got != "i18n:ui_hotkey_registration_failed" {
		t.Fatalf("generic registration message = %q", got)
	}
	if got := migrationApplyError(fmt.Errorf("portal: %w", keyboard.ErrHotkeyConflict)); got != migrationHotkeyTaken {
		t.Fatalf("confirmed conflict row = %q", got)
	}
}

type onboardingMigrationStub struct {
	id   string
	name string
}

func (s onboardingMigrationStub) ID() string            { return s.id }
func (s onboardingMigrationStub) Name() string          { return s.name }
func (s onboardingMigrationStub) Version() string       { return "" }
func (s onboardingMigrationStub) Location() string      { return "" }
func (s onboardingMigrationStub) Icon() common.WoxImage { return common.WoxImage{} }
func (s onboardingMigrationStub) Hotkey() string        { return "" }
func (s onboardingMigrationStub) Plugins(context.Context) ([]migrate.Plugin, error) {
	return nil, nil
}
func (s onboardingMigrationStub) Catalog(context.Context) ([]migrate.Category, error) {
	return nil, nil
}
func (s onboardingMigrationStub) Import(context.Context, []string) (migrate.ImportResult, error) {
	return migrate.ImportResult{}, nil
}

func TestOnboardingStepsStartWithIntroductionAndOmitAdvancedQuerySetup(t *testing.T) {
	steps := (&App{}).onboardingSteps()
	want := []string{"welcome", "mainHotkey"}
	if runtime.GOOS == "darwin" {
		want = []string{"welcome", "permissions", "mainHotkey"}
	}
	if len(steps) < len(want) {
		t.Fatalf("onboarding steps = %#v, want prefix %v", steps, want)
	}
	for index, id := range want {
		if steps[index].ID != id {
			t.Fatalf("onboarding step %d = %q, want %q in %#v", index, steps[index].ID, id, steps)
		}
	}
	for _, step := range steps {
		if step.ID == "selectionHotkey" || step.ID == "trayQueries" {
			t.Fatalf("onboarding includes removed step %q", step.ID)
		}
	}
}

func TestDefaultOnboardingQueryHotkeyUsesPlatformPrimaryModifier(t *testing.T) {
	want := "Ctrl+Shift+V"
	if runtime.GOOS == "darwin" {
		want = "Cmd+Shift+V"
	}
	if got := defaultOnboardingQueryHotkey(); got != want {
		t.Fatalf("default query hotkey = %q, want %q", got, want)
	}
}

func TestUpsertOnboardingQueryHotkeyPreservesOtherQueries(t *testing.T) {
	items := upsertOnboardingQueryHotkey([]queryHotkeySetting{
		{Hotkey: "Ctrl+G", Query: "github"},
		{Hotkey: "Ctrl+C", Query: "cb ", Disabled: true},
	}, "Ctrl+Shift+V")
	if len(items) != 2 || items[0].Hotkey != "Ctrl+G" || items[1].Hotkey != "Ctrl+Shift+V" || items[1].Disabled {
		t.Fatalf("query hotkeys = %#v", items)
	}
}

func TestRemoveOnboardingQueryHotkeyPreservesOtherQueries(t *testing.T) {
	items := removeOnboardingQueryHotkey([]queryHotkeySetting{{Hotkey: "Ctrl+G", Query: "github"}, {Hotkey: "Ctrl+V", Query: " cb "}})
	if len(items) != 1 || items[0].Query != "github" {
		t.Fatalf("query hotkeys = %#v", items)
	}
}

func TestRecordingConfiguredOnboardingQueryHotkeyPreservesReadyState(t *testing.T) {
	state := &onboardingQueryHotkeyState{selected: true, ready: true, saved: true}
	app := &App{onboardingQueryHotkey: state, hotkeySettings: newHotkeySettingsController(CommonDeps{})}

	app.recordOnboardingQueryHotkey()

	if !state.ready {
		t.Fatal("configured query hotkey became not ready when recording started")
	}
}

func TestOnboardingRecommendedPluginsUsesStableOrderAndPlatformFilter(t *testing.T) {
	plugins := []pluginSettingsPlugin{
		{ID: "8b8a1b35-3d9e-4d7d-9f2e-3b1d0b7f9e10", Name: "IP Geolocation"},
		{ID: "6987b7b1-89da-41ef-bab3-d1ba2e3daba0", Name: "Everything"},
		{ID: "0057ebd4-1a85-4653-8bfa-d51557c0c7a1", Name: "Unsplash"},
		{ID: "6dd42f91-009d-4d14-909c-97f25454eea7", Name: "Awake"},
	}
	windows := onboardingRecommendedPlugins(plugins, "windows")
	if len(windows) != 4 || windows[0].Name != "Awake" || windows[1].Name != "Everything" || windows[2].Name != "Unsplash" || windows[3].Name != "IP Geolocation" {
		t.Fatalf("Windows recommendations = %#v", windows)
	}
	linux := onboardingRecommendedPlugins(plugins, "linux")
	if len(linux) != 3 || linux[0].Name != "Awake" || linux[1].Name != "Unsplash" || linux[2].Name != "IP Geolocation" {
		t.Fatalf("Linux recommendations = %#v", linux)
	}
}

func TestOnboardingSystemThemesUsesBundledOrder(t *testing.T) {
	themes := []themeSettingsTheme{
		{ID: "532238bc-6eda-4011-a080-c365b67486fc", Name: "Wox Auto"},
		{ID: "92dc0ea7-a52f-4b0a-9f0d-7cb36a634860", Name: "Wox Light"},
		{ID: onboardingGlassID, Name: "Wox Glass"},
		{ID: "53c1d0a4-ffc8-4d90-91dc-b408fb0b9a03", Name: "Wox Dark"},
		{ID: "community", Name: "Community"},
	}
	got := onboardingSystemThemes(themes)
	if len(got) != 4 || got[0].Name != "Wox Glass" || got[1].Name != "Wox Dark" || got[2].Name != "Wox Light" || got[3].Name != "Wox Auto" {
		t.Fatalf("onboarding system themes = %#v", got)
	}
}

func TestOnboardingUsesBundledGlassPalette(t *testing.T) {
	data, err := resource.ThemeFS.ReadFile("themes/glass.json")
	if err != nil {
		t.Fatal(err)
	}
	var theme themeData
	if err := json.Unmarshal(data, &theme); err != nil {
		t.Fatal(err)
	}
	// The bundled theme has platform overrides; compare the resolved palette.
	if !reflect.DeepEqual(onboardingPreviewTheme, paletteForTheme(theme).componentTheme()) {
		t.Fatalf("onboarding theme = %#v, want bundled Wox Glass palette", onboardingPreviewTheme)
	}
}

func TestOnboardingCanContinueWhileThemeApplies(t *testing.T) {
	app := &App{onboardingTheme: onboardingThemeState{applying: true}, hotkeySettings: newHotkeySettingsController(CommonDeps{})}
	for index, step := range app.onboardingSteps() {
		if step.ID != "themeInstall" {
			continue
		}
		app.onboardingStep = index
		app.selectOnboardingStep(index + 1)
		if app.onboardingStep != index+1 {
			t.Fatalf("onboarding step = %d, want navigation to continue while the selected theme finishes applying", app.onboardingStep)
		}
		return
	}
	t.Fatal("theme onboarding step not found")
}
