package launcher

import (
	"runtime"
	"testing"
)

func TestOnboardingStepsStartWithIntroductionAndOmitAdvancedQuerySetup(t *testing.T) {
	steps := (&App{}).onboardingSteps()
	if len(steps) < 2 || steps[0].ID != "welcome" || steps[1].ID != "mainHotkey" {
		t.Fatalf("first onboarding steps = %#v, want welcome then main hotkey setup", steps)
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
		{ID: onboardingGlassDarkID, Name: "Wox Glass Dark"},
		{ID: "53c1d0a4-ffc8-4d90-91dc-b408fb0b9a03", Name: "Wox Dark"},
		{ID: "community", Name: "Community"},
	}
	got := onboardingSystemThemes(themes)
	if len(got) != 4 || got[0].Name != "Wox Glass Dark" || got[1].Name != "Wox Dark" || got[2].Name != "Wox Light" || got[3].Name != "Wox Auto" {
		t.Fatalf("onboarding system themes = %#v", got)
	}
}

func TestOnboardingUsesBundledGlassDarkPalette(t *testing.T) {
	background := onboardingGlassDarkTheme.Background
	if background.R != 22 || background.G != 22 || background.B != 26 || onboardingGlassDarkTheme.QueryBackground.A != 0 {
		t.Fatalf("onboarding theme = %#v, want bundled Wox Glass Dark palette", onboardingGlassDarkTheme)
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
