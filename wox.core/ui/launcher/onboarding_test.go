package launcher

import (
	"runtime"
	"testing"
)

func TestIncludeOnboardingSelectionHotkeyOmitsLinux(t *testing.T) {
	if includeOnboardingSelectionHotkey("linux") {
		t.Fatal("linux onboarding must omit selection hotkey because selection query is unsupported")
	}
	for _, goos := range []string{"windows", "darwin"} {
		if !includeOnboardingSelectionHotkey(goos) {
			t.Fatalf("%s onboarding must include selection hotkey", goos)
		}
	}
}

func TestIncludeOnboardingTrayQueriesOmitsLinux(t *testing.T) {
	if includeOnboardingTrayQueries("linux") {
		t.Fatal("linux onboarding must omit tray queries because tray query is unsupported")
	}
	for _, goos := range []string{"windows", "darwin"} {
		if !includeOnboardingTrayQueries(goos) {
			t.Fatalf("%s onboarding must include tray queries", goos)
		}
	}
}

func TestOnboardingStepsFollowPlatformUnsupportedFeatures(t *testing.T) {
	steps := (&App{}).onboardingSteps()
	hasSelection := false
	hasTrayQueries := false
	for _, step := range steps {
		switch step.ID {
		case "selectionHotkey":
			hasSelection = true
		case "trayQueries":
			hasTrayQueries = true
		}
	}
	if want := includeOnboardingSelectionHotkey(runtime.GOOS); hasSelection != want {
		t.Fatalf("selection hotkey step present = %v, want %v", hasSelection, want)
	}
	if want := includeOnboardingTrayQueries(runtime.GOOS); hasTrayQueries != want {
		t.Fatalf("tray queries step present = %v, want %v", hasTrayQueries, want)
	}
}
