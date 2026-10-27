package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"wox/common"
	"wox/plugin/thirdparty/migrate"
	"wox/resource"
	"wox/setting"
	"wox/ui/contract"
	"wox/ui/omarchytheme"
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
	app := bindTestApp(&App{
		services:        services,
		onboarding:      onboardingController{open: true},
		generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
		language:        languageCatalog{translations: map[string]string{"ui_hotkey_registration_unsupported": "Registration unavailable"}},
	})
	form := newFormFieldsState(nil, map[string]string{"MainHotkey": "Alt+K"}, true)
	state := &hotkeyRecordingState{diagnosticCtx: context.Background(), target: &form, persistKey: "MainHotkey"}
	app.hotkeyRecording.saveRecordedHotkeySetting(state, "MainHotkey", "Alt+K", "Ctrl+Space")
	if app.onboarding.pageError != "Registration unavailable" || form.values["MainHotkey"] != "Ctrl+Space" {
		t.Fatalf("failed save: error=%q hotkey=%q", app.onboarding.pageError, form.values["MainHotkey"])
	}
	services.err = nil
	app.hotkeyRecording.saveRecordedHotkeySetting(state, "MainHotkey", "Alt+K", "Ctrl+Space")
	if app.onboarding.pageError != "" || app.generalSettings.Data().MainHotkey != "Alt+K" {
		t.Fatalf("successful retry: error=%q hotkey=%q", app.onboarding.pageError, app.generalSettings.Data().MainHotkey)
	}
}

func TestOnboardingMigrationStepFollowsWelcome(t *testing.T) {
	plain := (bindTestApp(&App{})).onboarding.onboardingSteps()
	for _, step := range plain {
		if step.ID == "migrate" {
			t.Fatal("migration step appears without a detected launcher")
		}
	}
	if plain[len(plain)-1].ID != "finish" {
		t.Fatalf("last step = %q", plain[len(plain)-1].ID)
	}
	app := bindTestApp(&App{
		onboarding: onboardingController{
			migration: onboardingMigrationState{installations: []migrate.Installation{
				onboardingMigrationStub{id: "flow", name: "Flow Launcher"},
				onboardingMigrationStub{id: "other", name: "Other"},
			}},
		},
	})
	steps := app.onboarding.onboardingSteps()
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
	app.onboarding.step = wantIndex
	app.onboarding.migration.selectedID = "flow"
	app.onboarding.selectOnboardingStep(wantIndex - 1)
	if app.onboarding.step != wantIndex || !app.onboarding.migration.choosing || app.onboarding.migration.selectedID != "" {
		t.Fatalf("back from items step=%d choosing=%v selected=%q", app.onboarding.step, app.onboarding.migration.choosing, app.onboarding.migration.selectedID)
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

type migrationSettingWriter struct {
	contract.Services
	values map[string]string
}

func (s *migrationSettingWriter) UpdateGeneralSetting(_ context.Context, _, key, value string) error {
	if s.values == nil {
		s.values = map[string]string{}
	}
	s.values[key] = value
	return nil
}

type pluginSettingMigrationWriter struct {
	contract.Services
	pluginID string
	values   map[string]string
	general  string
}

func (s *pluginSettingMigrationWriter) UpdatePluginSettings(_ context.Context, _, pluginID string, values map[string]string) error {
	s.pluginID = pluginID
	s.values = values
	return nil
}

func (s *pluginSettingMigrationWriter) UpdateGeneralSetting(_ context.Context, _, key, value string) error {
	s.general = key + "=" + value
	return nil
}

func TestMigrationPluginSettingUsesThePluginStore(t *testing.T) {
	services := &pluginSettingMigrationWriter{}
	app := bindTestApp(&App{
		services:  services,
		sessionID: "session",
	})
	err := app.onboarding.applyOnboardingMigrationSetting(context.Background(), migrate.SettingWrite{
		Key: "ignorePatterns", Value: `[{"Pattern":"**/AppData/**"}]`, PluginID: common.FileSearchPluginID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if services.pluginID != common.FileSearchPluginID || services.values["ignorePatterns"] != `[{"Pattern":"**/AppData/**"}]` || services.general != "" {
		t.Fatalf("plugin=%s values=%v general=%q", services.pluginID, services.values, services.general)
	}
}

type migrationThemeInstaller struct {
	contract.Services
	theme common.Theme
}

func (s *migrationThemeInstaller) InstallMigratedTheme(_ context.Context, _ string, theme common.Theme) error {
	s.theme = theme
	return nil
}

func TestMigratedThemeIsInstalledAndSelected(t *testing.T) {
	var theme common.Theme
	if err := json.Unmarshal([]byte(`{"SchemaVersion":2,"ThemeId":"flow-cyan","ThemeName":"Cyan Dark","BaseBackgroundColor":"#0F1F26","BaseTextColor":"#E7E9EB","BaseAccentColor":"#58C2B4"}`), &theme); err != nil {
		t.Fatal(err)
	}
	services := &migrationThemeInstaller{}
	app := bindTestApp(&App{
		services:        services,
		generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
	})
	err := app.onboarding.applyMigrationWriteGroup(context.Background(), &migrationWriteGroup{themes: []migrate.ThemeWrite{{
		ItemIDs: []string{"setting:theme"},
		Theme:   theme,
	}}})
	if err != "" {
		t.Fatal(err)
	}
	if services.theme.ThemeId != "flow-cyan" || app.generalSettings.Data().ThemeID != "flow-cyan" {
		t.Fatalf("installed %#v selected %s", services.theme.ThemeId, app.generalSettings.Data().ThemeID)
	}
	for _, step := range app.onboarding.onboardingSteps() {
		if step.ID == "themeInstall" {
			t.Fatal("theme step still shown after a launcher theme was imported")
		}
	}
}

func TestImportedQueryHotkeysAreAppended(t *testing.T) {
	services := &migrationSettingWriter{}
	app := bindTestApp(&App{
		services:        services,
		generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
		hotkeySettings:  newHotkeySettingsController(CommonDeps{}),
	})
	app.generalSettings.ApplyData(settingsData{QueryHotkeys: []queryHotkeySetting{{
		Hotkey: "Ctrl+Shift+V", Query: "cb ", Position: "system_default",
	}}})
	form := newFormFieldsState(nil, map[string]string{"QueryHotkeys": `[{"Hotkey":"Ctrl+Shift+V","Query":"cb "}]`}, true)
	app.hotkeySettings.SetForm(&form)

	err := app.onboarding.applyOnboardingMigrationSetting(context.Background(), migrate.SettingWrite{
		Key:   "QueryHotkeys",
		Value: `[{"Hotkey":"Ctrl+Shift+E","Query":"test on flow"}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := app.generalSettings.Data().QueryHotkeys
	if len(got) != 2 || got[0].Query != "cb " || got[1].Hotkey != "Ctrl+Shift+E" || got[1].Query != "test on flow" || got[1].Position != "system_default" {
		t.Fatalf("query hotkeys = %+v", got)
	}
	stored := services.values["QueryHotkeys"]
	if !strings.Contains(stored, `"Query":"cb "`) || !strings.Contains(stored, `"Query":"test on flow"`) {
		t.Fatalf("stored %s", stored)
	}
	if form.values["QueryHotkeys"] != stored {
		t.Fatalf("form %s", form.values["QueryHotkeys"])
	}
}

func TestImportedQueryHotkeyKeepsTheExistingChord(t *testing.T) {
	services := &migrationSettingWriter{}
	app := bindTestApp(&App{
		services:        services,
		generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
	})
	app.generalSettings.ApplyData(settingsData{QueryHotkeys: []queryHotkeySetting{{
		Hotkey: "Ctrl+Shift+E", Query: "keep",
	}}})
	err := app.onboarding.applyOnboardingMigrationSetting(context.Background(), migrate.SettingWrite{
		Key:   "QueryHotkeys",
		Value: `[{"Hotkey":"Ctrl + Shift + E","Query":"replace"}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := app.generalSettings.Data().QueryHotkeys
	if len(got) != 1 || got[0].Query != "keep" {
		t.Fatalf("query hotkeys = %+v", got)
	}
	if _, wrote := services.values["QueryHotkeys"]; wrote {
		t.Fatalf("stored %s", services.values["QueryHotkeys"])
	}
}

func TestImportedQueryAliasesAreAppended(t *testing.T) {
	services := &migrationSettingWriter{}
	app := bindTestApp(&App{
		services:        services,
		generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
	})
	app.generalSettings.ApplyData(settingsData{QueryAliases: []queryAliasSetting{{
		Alias: "hw", Query: "hello world",
	}}})
	form := newFormFieldsState(nil, map[string]string{"QueryAliases": `[{"Shortcut":"hw","Query":"hello world"}]`}, true)
	app.generalSettings.SetForm(&form)

	err := app.onboarding.applyOnboardingMigrationSetting(context.Background(), migrate.SettingWrite{
		Key:   "QueryAliases",
		Value: `[{"Shortcut":"HW","Query":"other"},{"Shortcut":"gg","Query":"google"}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := app.generalSettings.Data().QueryAliases
	if len(got) != 2 || got[0].Alias != "hw" || got[0].Query != "hello world" || got[1].Alias != "gg" || got[1].Query != "google" {
		t.Fatalf("aliases = %+v", got)
	}
	if !strings.Contains(services.values["QueryAliases"], `"Shortcut":"hw"`) || !strings.Contains(services.values["QueryAliases"], `"Shortcut":"gg"`) {
		t.Fatalf("stored %s", services.values["QueryAliases"])
	}
	if strings.Contains(services.values["QueryAliases"], `"Query":"other"`) {
		t.Fatalf("stored replaced an alias: %s", services.values["QueryAliases"])
	}
}

func TestStagedMigrationHotkeyIsShownBeforeRegistration(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	services := &blockingMigrationHotkeyServices{started: started, release: release}
	form := newFormFieldsState(nil, map[string]string{"MainHotkey": "alt+space"}, true)
	app := bindTestApp(&App{
		services:        services,
		lifecycleCtx:    context.Background(),
		generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
		hotkeySettings:  newHotkeySettingsController(CommonDeps{}),
	})
	app.hotkeySettings.SetForm(&form)
	app.onboarding.migration.stagedMainHotkey = "ctrl+shift+space"
	app.onboarding.applyStagedMigrationHotkey()
	if form.values["MainHotkey"] != "ctrl+shift+space" {
		t.Fatalf("hotkey before registration = %q", form.values["MainHotkey"])
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("registration did not start")
	}
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for app.onboarding.migration.stagedMainHotkeyError == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if app.onboarding.migration.stagedMainHotkeyError == "" || form.values["MainHotkey"] != "ctrl+shift+space" {
		t.Fatalf("after registration hotkey=%q error=%q", form.values["MainHotkey"], app.onboarding.migration.stagedMainHotkeyError)
	}
}

func TestSettingsReloadKeepsTheStagedMigrationHotkey(t *testing.T) {
	data := overlayStagedMigrationHotkey(settingsData{MainHotkey: "alt+space"}, onboardingMigrationState{
		stagedMainHotkey:      "ctrl+shift+space",
		stagedMainHotkeyError: "i18n:onboarding_migrate_hotkey_next",
	})
	if data.MainHotkey != "ctrl+shift+space" || !data.MainHotkeyRegistrationFailed || data.MainHotkeyRegistrationError != "i18n:onboarding_migrate_hotkey_next" {
		t.Fatalf("reloaded hotkey = %+v", data.MainHotkey)
	}
	plain := overlayStagedMigrationHotkey(settingsData{MainHotkey: "alt+space"}, onboardingMigrationState{})
	if plain.MainHotkey != "alt+space" || plain.MainHotkeyRegistrationFailed {
		t.Fatalf("unstaged reload = %+v", plain.MainHotkey)
	}
}

type blockingMigrationHotkeyServices struct {
	contract.Services
	started chan struct{}
	release chan struct{}
}

func (s *blockingMigrationHotkeyServices) UpdateGeneralSetting(context.Context, string, string, string) error {
	close(s.started)
	<-s.release
	return fmt.Errorf("hotkey already registered: 1409")
}

type currentMainHotkeyServices struct {
	contract.Services
	saved string
}

func (s *currentMainHotkeyServices) CheckHotkeyAvailability(context.Context, string, string) (contract.HotkeyAvailability, error) {
	return contract.HotkeyAvailability{Available: false, ConflictType: "main"}, nil
}

func (s *currentMainHotkeyServices) UpdateGeneralSetting(_ context.Context, _, key, value string) error {
	s.saved = key + "=" + value
	return nil
}

func TestRecordingTheCurrentMainHotkeyIsNotASelfConflict(t *testing.T) {
	services := &currentMainHotkeyServices{}
	form := newFormFieldsState(
		[]formDefinition{{Type: "hotkey", Value: formDefinitionValue{Key: "MainHotkey"}}},
		map[string]string{"MainHotkey": "ctrl+shift+space"},
		true,
	)
	app := bindTestApp(&App{
		services:        services,
		onboarding:      onboardingController{open: true},
		lifecycleCtx:    context.Background(),
		generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
		hotkeySettings:  newHotkeySettingsController(CommonDeps{}),
		pluginSettings:  newPluginSettingsController(CommonDeps{}),
		language:        languageCatalog{translations: map[string]string{"ui_hotkey_conflict_main": "used by the main hotkey"}},
	})
	app.generalSettings.ApplyData(settingsData{MainHotkey: "alt+space"})
	app.hotkeySettings.SetForm(&form)
	state := &hotkeyRecordingState{
		diagnosticCtx: context.Background(), target: app.hotkeySettings.Form(), persistKey: "MainHotkey",
	}
	app.hotkeyRecording.SetRecording(state)
	app.hotkeyRecording.checkRecordedHotkey(state, "alt+space")
	if form.values["MainHotkey"] != "alt+space" || state.statusError || state.status == "used by the main hotkey" {
		t.Fatalf("value=%q error=%v status=%q", form.values["MainHotkey"], state.statusError, state.status)
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

type exportInstallStub struct {
	onboardingMigrationStub
	resets     int
	generation uint64
}

func (s *exportInstallStub) Recognize(string) (bool, string) { return true, "" }
func (s *exportInstallStub) Unlock(context.Context, string, string) error {
	s.generation++
	return nil
}
func (s *exportInstallStub) Reset() {
	s.resets++
	s.generation++
}
func (s *exportInstallStub) Generation() uint64               { return s.generation }
func (s *exportInstallStub) SetKeepKeyword(string, bool) bool { return false }

func TestLeavingMigrationReleasesTheExport(t *testing.T) {
	source := &exportInstallStub{onboardingMigrationStub: onboardingMigrationStub{id: "raycast", name: "Raycast"}}
	app := bindTestApp(&App{
		onboarding:     onboardingController{migration: newOnboardingMigrationState([]migrate.Installation{source})},
		hotkeySettings: newHotkeySettingsController(CommonDeps{}),
	})
	migrateIndex := -1
	for index, step := range app.onboarding.onboardingSteps() {
		if step.ID == "migrate" {
			migrateIndex = index
		}
	}
	if migrateIndex < 0 {
		t.Fatal("migration step missing")
	}
	app.onboarding.step = migrateIndex
	app.onboarding.selectOnboardingStep(migrateIndex + 1)
	if source.resets != 1 || app.onboarding.migration.unlocked {
		t.Fatalf("resets=%d unlocked=%v", source.resets, app.onboarding.migration.unlocked)
	}
	app.onboarding.releaseOnboardingMigrationSession()
	if source.resets != 2 {
		t.Fatalf("second release resets=%d", source.resets)
	}
}

type orderedImportInstall struct {
	onboardingMigrationStub
}

func (orderedImportInstall) Import(_ context.Context, ids []string) (migrate.ImportResult, error) {
	result := migrate.ImportResult{}
	for _, id := range ids {
		switch id {
		case "quicklink:1":
			result.Quicklinks = append(result.Quicklinks, migrate.QuicklinkWrite{ItemID: id, Title: "wox", Keyword: "wox", URL: "https://example.com"})
		case migrate.ClipboardSummaryID:
			result.Clipboard = append(result.Clipboard, migrate.ClipboardWrite{ItemID: id, Text: "copied"})
		case "setting:autostart":
			result.Settings = append(result.Settings, migrate.SettingWrite{ItemIDs: []string{id}, Key: "EnableAutostart", Value: "true"})
		}
	}
	return result, nil
}

type orderedImportServices struct {
	contract.Services
	active func() string
	seen   []string
}

func (s *orderedImportServices) note(id string) {
	s.seen = append(s.seen, s.active()+"="+id)
}

func (s *orderedImportServices) ImportMigratedQuicklinks(_ context.Context, _ string, items []contract.MigratedQuicklink) []contract.MigratedItemResult {
	if len(items) == 0 {
		return nil
	}
	s.note(items[0].ItemID)
	return []contract.MigratedItemResult{{ID: items[0].ItemID, Status: "imported"}}
}

func (s *orderedImportServices) ImportMigratedFavorites(context.Context, string, []contract.MigratedFavorite) []contract.MigratedItemResult {
	return nil
}

func (s *orderedImportServices) ImportMigratedNotes(context.Context, string, []contract.MigratedNote) []contract.MigratedItemResult {
	return nil
}

func (s *orderedImportServices) ImportMigratedResultBindings(context.Context, string, []contract.MigratedResultBinding) []contract.MigratedItemResult {
	return nil
}

func (s *orderedImportServices) ImportMigratedClipboard(_ context.Context, _ string, items []contract.MigratedClipboard) []contract.MigratedItemResult {
	if len(items) == 0 {
		return nil
	}
	s.note(items[0].ItemID)
	return []contract.MigratedItemResult{{ID: items[0].ItemID, Status: "imported"}}
}

func (s *orderedImportServices) UpdateGeneralSetting(context.Context, string, string, string) error {
	s.note("setting:autostart")
	return nil
}

func TestImportShowsEachRowBeforeWritingIt(t *testing.T) {
	install := orderedImportInstall{onboardingMigrationStub: onboardingMigrationStub{id: "raycast", name: "Raycast"}}
	services := &orderedImportServices{}
	app := bindTestApp(&App{
		lifecycleCtx:   context.Background(),
		services:       services,
		onboarding:     onboardingController{migration: newOnboardingMigrationState([]migrate.Installation{install})},
		hotkeySettings: newHotkeySettingsController(CommonDeps{}),
	})
	app.onboarding.migration.categories = []onboardingMigrationCategory{
		{id: "quicklinks", items: []onboardingMigrationItem{{item: migrate.Item{ID: "quicklink:1", Kind: migrate.ItemKindQuicklink, Selectable: true}, selected: true}}},
		{id: "clipboard", items: []onboardingMigrationItem{{item: migrate.Item{ID: migrate.ClipboardSummaryID, Kind: migrate.ItemKindClipboard, Selectable: true}, selected: true}}},
		{id: "general", items: []onboardingMigrationItem{{item: migrate.Item{ID: "setting:autostart", Kind: migrate.ItemKindSetting, Selectable: true}, selected: true}}},
	}
	services.active = func() string { return app.onboarding.migration.activeID }
	done := make(chan struct{})
	var once sync.Once
	app.uiCall = func(fn func()) error {
		fn()
		if app.onboarding.migration.done {
			once.Do(func() { close(done) })
		}
		return nil
	}
	app.onboarding.importOnboardingMigration()
	if app.onboarding.migration.activeID != "quicklink:1" || !app.onboarding.migration.importing {
		t.Fatalf("click did not mark the first row active: %+v", app.onboarding.migration.activeID)
	}
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("import did not finish")
	}
	want := []string{"quicklink:1=quicklink:1", migrate.ClipboardSummaryID + "=" + migrate.ClipboardSummaryID, "setting:autostart=setting:autostart"}
	if strings.Join(services.seen, ",") != strings.Join(want, ",") {
		t.Fatalf("write order = %#v", services.seen)
	}
	for _, category := range app.onboarding.migration.categories {
		for _, item := range category.items {
			if !item.copied {
				t.Fatalf("row %s was not marked imported", item.item.ID)
			}
		}
	}
}

func TestMigrationDisplayWaitStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	waitOnboardingMigrationDisplay(ctx, started)
	if time.Since(started) > 50*time.Millisecond {
		t.Fatal("cancelled display wait blocked")
	}
}

func TestOnboardingStepsStartWithIntroductionAndOmitAdvancedQuerySetup(t *testing.T) {
	steps := (bindTestApp(&App{})).onboarding.onboardingSteps()
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
	switch runtime.GOOS {
	case "darwin":
		want = "Cmd+Shift+V"
	case "linux":
		want = "Ctrl+Alt+V"
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
	app := bindTestApp(&App{
		onboarding:     onboardingController{queryHotkey: state},
		hotkeySettings: newHotkeySettingsController(CommonDeps{}),
	})

	app.onboarding.recordOnboardingQueryHotkey()

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
	if onboardingGlassID != setting.DefaultThemeId {
		t.Fatalf("onboarding glass id = %s, default theme = %s", onboardingGlassID, setting.DefaultThemeId)
	}
}

func TestOnboardingReplacesWoxAutoWithOmarchyAuto(t *testing.T) {
	themes := []themeSettingsTheme{
		{ID: "532238bc-6eda-4011-a080-c365b67486fc", Name: "Wox Auto"},
		{ID: "92dc0ea7-a52f-4b0a-9f0d-7cb36a634860", Name: "Wox Light"},
		{ID: onboardingGlassID, Name: "Wox Glass"},
		{ID: "53c1d0a4-ffc8-4d90-91dc-b408fb0b9a03", Name: "Wox Dark"},
		{ID: omarchytheme.ThemeID, Name: omarchytheme.ThemeName},
	}
	got := onboardingSystemThemes(themes)
	if len(got) != 4 || got[0].Name != "Wox Glass" || got[1].Name != "Wox Dark" || got[2].Name != "Wox Light" {
		t.Fatalf("onboarding themes = %#v", got)
	}
	want := "Wox Auto"
	if omarchytheme.Available() {
		want = "Omarchy Auto"
	}
	if got[3].Name != want {
		t.Fatalf("auto theme = %s, want %s", got[3].Name, want)
	}
}

type onboardingThemeApplyStub struct {
	contract.Services
	mu sync.Mutex
	id string
}

func (s *onboardingThemeApplyStub) OperateTheme(_ context.Context, _ string, themeID string, _ contract.ThemeOperation) error {
	s.mu.Lock()
	s.id = themeID
	s.mu.Unlock()
	return nil
}

func (s *onboardingThemeApplyStub) CurrentTheme(context.Context, string) (common.Theme, error) {
	return common.Theme{ThemeId: omarchytheme.ThemeID, ThemeName: omarchytheme.ThemeName}, nil
}

func (s *onboardingThemeApplyStub) applied() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id
}

func TestOnboardingThemeStepAppliesOmarchyAutoWhenStillDefault(t *testing.T) {
	if !omarchytheme.Available() {
		t.Skip("Omarchy Auto is offered only when Omarchy is installed")
	}
	services := &onboardingThemeApplyStub{}
	app := bindTestApp(&App{
		services:        services,
		lifecycleCtx:    context.Background(),
		generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
		themeSettings:   newThemeSettingsController(CommonDeps{}),
		hotkeySettings:  newHotkeySettingsController(CommonDeps{}),
	})
	app.appearance.applyTheme = func(themeData) {}
	app.generalSettings.Update(func(data *settingsData) { data.ThemeID = onboardingGlassID })
	app.themeSettings.SetThemes([]themeSettingsTheme{
		{ID: onboardingGlassID, Name: "Wox Glass"},
		{ID: "53c1d0a4-ffc8-4d90-91dc-b408fb0b9a03", Name: "Wox Dark"},
		{ID: "92dc0ea7-a52f-4b0a-9f0d-7cb36a634860", Name: "Wox Light"},
		{ID: "532238bc-6eda-4011-a080-c365b67486fc", Name: "Wox Auto"},
		{ID: omarchytheme.ThemeID, Name: omarchytheme.ThemeName},
	})
	enterOnboardingThemeStep(t, app)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && (app.onboarding.themes.applying || app.generalSettings.Data().ThemeID != omarchytheme.ThemeID) {
		time.Sleep(10 * time.Millisecond)
	}
	if app.generalSettings.Data().ThemeID != omarchytheme.ThemeID || services.applied() != omarchytheme.ThemeID || app.onboarding.themes.applying || app.onboarding.themes.error != "" {
		t.Fatalf("theme = %s applied = %s applying=%v error=%s", app.generalSettings.Data().ThemeID, services.applied(), app.onboarding.themes.applying, app.onboarding.themes.error)
	}
	items := onboardingThemeItems(app.themeSettings.Themes(), app.generalSettings.Data().ThemeID)
	if len(items) != 4 || items[3].ID != omarchytheme.ThemeID || !items[3].Selected || items[0].Name != "Wox Glass" {
		t.Fatalf("theme cards = %#v", items)
	}
	services.mu.Lock()
	services.id = ""
	services.mu.Unlock()
	enterOnboardingThemeStep(t, app)
	time.Sleep(30 * time.Millisecond)
	if services.applied() != "" || app.generalSettings.Data().ThemeID != omarchytheme.ThemeID {
		t.Fatalf("second visit applied %s theme %s", services.applied(), app.generalSettings.Data().ThemeID)
	}

	app.generalSettings.Update(func(data *settingsData) { data.ThemeID = onboardingGlassID })
	enterOnboardingThemeStep(t, app)
	time.Sleep(30 * time.Millisecond)
	if services.applied() != "" || app.generalSettings.Data().ThemeID != onboardingGlassID {
		t.Fatalf("explicit Glass was replaced: applied %s theme %s", services.applied(), app.generalSettings.Data().ThemeID)
	}
}

func TestOnboardingThemeStepKeepsAnExplicitTheme(t *testing.T) {
	catalog := []themeSettingsTheme{
		{ID: onboardingGlassID, Name: "Wox Glass"},
		{ID: "53c1d0a4-ffc8-4d90-91dc-b408fb0b9a03", Name: "Wox Dark"},
		{ID: "92dc0ea7-a52f-4b0a-9f0d-7cb36a634860", Name: "Wox Light"},
		{ID: "532238bc-6eda-4011-a080-c365b67486fc", Name: "Wox Auto"},
		{ID: omarchytheme.ThemeID, Name: omarchytheme.ThemeName},
	}
	app := bindTestApp(&App{
		services:        &migrationSettingWriter{},
		generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
		themeSettings:   newThemeSettingsController(CommonDeps{}),
		hotkeySettings:  newHotkeySettingsController(CommonDeps{}),
	})
	app.generalSettings.Update(func(data *settingsData) { data.ThemeID = "53c1d0a4-ffc8-4d90-91dc-b408fb0b9a03" })
	app.themeSettings.SetThemes(catalog)
	enterOnboardingThemeStep(t, app)
	if app.generalSettings.Data().ThemeID != "53c1d0a4-ffc8-4d90-91dc-b408fb0b9a03" || app.onboarding.themes.applying {
		t.Fatalf("chosen theme = %s applying=%v", app.generalSettings.Data().ThemeID, app.onboarding.themes.applying)
	}
}

func enterOnboardingThemeStep(t *testing.T, app *App) {
	t.Helper()
	for index, step := range app.onboarding.onboardingSteps() {
		if step.ID != "themeInstall" {
			continue
		}
		app.onboarding.selectOnboardingStep(index)
		return
	}
	t.Fatal("theme onboarding step not found")
}

// TestOnboardingThemeSelectionFollowsCurrentSetting covers a catalog cached before
// migration and navigation without clicking a card, including returning to the page.
func TestOnboardingThemeSelectionFollowsCurrentSetting(t *testing.T) {
	catalog := []themeSettingsTheme{
		{ID: onboardingGlassID, Name: "Wox Glass"},
		{ID: "53c1d0a4-ffc8-4d90-91dc-b408fb0b9a03", Name: "Wox Dark"},
		{ID: "92dc0ea7-a52f-4b0a-9f0d-7cb36a634860", Name: "Wox Light"},
		{ID: "532238bc-6eda-4011-a080-c365b67486fc", Name: "Wox Auto", IsAuto: true,
			LightThemeID: "92dc0ea7-a52f-4b0a-9f0d-7cb36a634860", DarkThemeID: "53c1d0a4-ffc8-4d90-91dc-b408fb0b9a03"},
	}
	for _, theme := range append(catalog, themeSettingsTheme{ID: "custom", Name: "Custom"}) {
		for _, imported := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/imported=%t", theme.Name, imported), func(t *testing.T) {
				services := &migrationSettingWriter{}
				app := bindTestApp(&App{
					services:        services,
					generalSettings: newGeneralSettingsController(CommonDeps{}, newSharedEditState()),
					themeSettings:   newThemeSettingsController(CommonDeps{}),
					hotkeySettings:  newHotkeySettingsController(CommonDeps{}),
				})
				app.generalSettings.Update(func(data *settingsData) { data.ThemeID = onboardingGlassID })
				app.themeSettings.SetThemes(catalog)
				app.onboarding.loadOnboardingThemes()
				if imported {
					// Raycast appearance is imported as a general ThemeId setting.
					if err := app.onboarding.applyOnboardingMigrationSetting(context.Background(), migrate.SettingWrite{Key: "ThemeId", Value: theme.ID}); err != nil {
						t.Fatal(err)
					}
				} else {
					app.generalSettings.Update(func(data *settingsData) { data.ThemeID = theme.ID })
				}
				assertSelection := func() {
					t.Helper()
					items := onboardingThemeItems(app.themeSettings.Snapshot().Themes, app.generalSettings.Snapshot().Data.ThemeID)
					var selected []string
					for _, item := range items {
						if item.Selected {
							selected = append(selected, item.ID)
						}
					}
					var want []string
					if theme.ID != "custom" {
						want = []string{theme.ID}
					}
					if !reflect.DeepEqual(selected, want) || app.themeSettings.currentThemeID() != theme.ID {
						t.Fatalf("selected = %v, active = %s; want %v, %s", selected, app.themeSettings.currentThemeID(), want, theme.ID)
					}
				}
				for index, step := range app.onboarding.onboardingSteps() {
					if step.ID != "themeInstall" {
						continue
					}
					app.onboarding.step = index
					app.onboarding.selectOnboardingStep(index)
					assertSelection()
					app.onboarding.selectOnboardingStep(index + 1)
					if app.onboarding.step != index+1 {
						t.Fatal("Continue did not advance from the theme page")
					}
					assertSelection()
					app.onboarding.selectOnboardingStep(index)
					assertSelection()
					if imported && services.values["ThemeId"] != theme.ID {
						t.Fatalf("persisted theme = %s, want %s", services.values["ThemeId"], theme.ID)
					}
					return
				}
				t.Fatal("theme onboarding step not found")
			})
		}
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
	app := bindTestApp(&App{
		onboarding:     onboardingController{themes: onboardingThemeState{applying: true}},
		hotkeySettings: newHotkeySettingsController(CommonDeps{}),
	})
	for index, step := range app.onboarding.onboardingSteps() {
		if step.ID != "themeInstall" {
			continue
		}
		app.onboarding.step = index
		app.onboarding.selectOnboardingStep(index + 1)
		if app.onboarding.step != index+1 {
			t.Fatalf("onboarding step = %d, want navigation to continue while the selected theme finishes applying", app.onboarding.step)
		}
		return
	}
	t.Fatal("theme onboarding step not found")
}

func TestClipboardSummaryOutcomeCollapsesEntryResults(t *testing.T) {
	ok := clipboardSummaryOutcome([]contract.MigratedItemResult{
		{ID: "clipboard:1", Status: "imported"},
		{ID: "clipboard:2", Status: "exists"},
		{ID: "clipboard:3", Status: "failed", ErrorCode: "write"},
	})
	if ok.ID != migrate.ClipboardSummaryID || ok.Status != "imported" || ok.ErrorCode != "" {
		t.Fatalf("partial success = %+v", ok)
	}
	failed := clipboardSummaryOutcome([]contract.MigratedItemResult{
		{ID: "clipboard:1", Status: "failed", ErrorCode: "write"},
	})
	if failed.Status != "failed" || failed.ErrorCode != "write" {
		t.Fatalf("failure = %+v", failed)
	}
}

type closeLauncherStub struct{ name string }

func (c closeLauncherStub) ID() string            { return "raycast" }
func (c closeLauncherStub) Name() string          { return c.name }
func (c closeLauncherStub) Version() string       { return "" }
func (c closeLauncherStub) Location() string      { return "" }
func (c closeLauncherStub) Icon() common.WoxImage { return common.WoxImage{} }
func (c closeLauncherStub) Hotkey() string        { return "" }
func (c closeLauncherStub) Plugins(context.Context) ([]migrate.Plugin, error) {
	return nil, nil
}
func (c closeLauncherStub) Catalog(context.Context) ([]migrate.Category, error) {
	return nil, nil
}
func (c closeLauncherStub) Import(context.Context, []string) (migrate.ImportResult, error) {
	return migrate.ImportResult{}, nil
}
func (c closeLauncherStub) CloseRunning(context.Context) error { return nil }

func TestMigrationCatalogStartsByClosingTheLauncher(t *testing.T) {
	catalog := onboardingMigrationCategories(closeLauncherStub{name: "Raycast"}, []migrate.Category{{
		ID:    migrate.CategoryHotkeys,
		Items: []migrate.Item{{ID: "setting:main_hotkey", TitleKey: "main_hotkey", Selectable: true}},
	}})
	if len(catalog) != 2 || catalog[0].id != migrate.CategoryPrepare || catalog[1].id != migrate.CategoryHotkeys {
		t.Fatalf("categories = %+v", catalog)
	}
	row := catalog[0].items[0]
	if !row.selected || !row.item.Required || row.item.Detail != "Raycast" || row.item.Kind != migrate.ItemKindCloseLauncher {
		t.Fatalf("close row = %+v", row)
	}
	if !migrationItemChosen(row) {
		t.Fatal("close row is not imported")
	}
	plain := catalog[1].items[0]
	plain.item.Selectable = false
	plain.selected = false
	if migrationItemChosen(plain) {
		t.Fatal("unselected row was imported")
	}
}

func TestOnboardingSessionStartsFreshAndLeavesOnce(t *testing.T) {
	app := bindTestApp(&App{})
	c := &app.onboarding
	c.step = 4
	c.pageError = "stale"
	c.themes.applying = true
	c.beginSessionLocked(nil)
	if !c.open || c.step != 0 || c.pageError != "" || c.themes.applying {
		t.Fatalf("new session = open %t step %d error %q applying %t, want a fresh first page", c.open, c.step, c.pageError, c.themes.applying)
	}

	c.choice = "language"
	if !c.leaveLocked() {
		t.Fatal("leaveLocked must report the open guide")
	}
	if c.open || c.choice != "" {
		t.Fatalf("after leave: open %t choice %q, want closed with no choice menu", c.open, c.choice)
	}
	if c.leaveLocked() {
		t.Fatal("leaving twice must not report an open guide")
	}
}
