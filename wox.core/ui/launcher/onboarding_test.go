package launcher

import (
	"testing"
)

func TestOnboardingStepsOmitAdvancedQuerySetup(t *testing.T) {
	steps := (&App{}).onboardingSteps()
	for _, step := range steps {
		if step.ID == "selectionHotkey" || step.ID == "trayQueries" {
			t.Fatalf("onboarding includes removed step %q", step.ID)
		}
	}
}
