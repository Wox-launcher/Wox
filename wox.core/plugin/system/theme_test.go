package system

import "testing"

func TestInstalledThemeListGroupOrdersSystemBelowCurrent(t *testing.T) {
	currentGroup, currentScore := installedThemeListGroup(true, true, "current", "system", "available")
	if currentGroup != "current" || currentScore != 100 {
		t.Fatalf("current system theme: group=%q score=%d", currentGroup, currentScore)
	}

	systemGroup, systemScore := installedThemeListGroup(false, true, "current", "system", "available")
	if systemGroup != "system" || systemScore != 75 {
		t.Fatalf("other system theme: group=%q score=%d", systemGroup, systemScore)
	}

	availableGroup, availableScore := installedThemeListGroup(false, false, "current", "system", "available")
	if availableGroup != "available" || availableScore != 50 {
		t.Fatalf("user theme: group=%q score=%d", availableGroup, availableScore)
	}

	if !(currentScore > systemScore && systemScore > availableScore) {
		t.Fatalf("group order scores: current=%d system=%d available=%d", currentScore, systemScore, availableScore)
	}
}
