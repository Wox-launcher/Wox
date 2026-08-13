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

func TestOnboardingStepsFollowPlatformSelectionHotkeySupport(t *testing.T) {
	steps := (&App{}).onboardingSteps()
	hasSelection := false
	for _, step := range steps {
		if step.ID == "selectionHotkey" {
			hasSelection = true
			break
		}
	}
	if want := includeOnboardingSelectionHotkey(runtime.GOOS); hasSelection != want {
		t.Fatalf("selection hotkey step present = %v, want %v", hasSelection, want)
	}
}
