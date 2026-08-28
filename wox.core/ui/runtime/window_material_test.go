package woxui

import "testing"

func TestWindowUsesDefaultMaterialExceptScreenshot(t *testing.T) {
	if !windowUsesDefaultMaterial(WindowRoleUtility) || !windowUsesDefaultMaterial(WindowRoleApplication) {
		t.Fatal("utility and application windows must inherit the process material")
	}
	if windowUsesDefaultMaterial(WindowRoleScreenshot) {
		t.Fatal("screenshot windows must keep showing the desktop")
	}
}

func TestSetDefaultAppearanceIsProcessWide(t *testing.T) {
	previous := DefaultAppearanceIsDark()
	t.Cleanup(func() { SetDefaultAppearance(previous) })

	SetDefaultAppearance(false)
	if DefaultAppearanceIsDark() {
		t.Fatal("default appearance should be light")
	}
	SetDefaultAppearance(true)
	if !DefaultAppearanceIsDark() {
		t.Fatal("default appearance should be dark")
	}
}
