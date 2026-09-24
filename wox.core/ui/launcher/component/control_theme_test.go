package component

import "testing"

func TestControlThemeScaledKeepsNormalDensity(t *testing.T) {
	if got := (ControlTheme{}).Scaled(SettingsLabelFontSize); got != SettingsLabelFontSize {
		t.Fatalf("missing scale = %v, want %v", got, SettingsLabelFontSize)
	}
	if got := (ControlTheme{DensityScale: 1}).Scaled(SettingsPageTitleFontSize); got != SettingsPageTitleFontSize {
		t.Fatalf("normal scale = %v, want %v", got, SettingsPageTitleFontSize)
	}
}

func TestControlThemeScaledFollowsInterfaceSize(t *testing.T) {
	comfortable := ControlTheme{DensityScale: 1.1}
	if got := comfortable.Scaled(SettingsLabelFontSize); got != 14 {
		t.Fatalf("comfortable label = %v, want 14", got)
	}
	if got := comfortable.Scaled(SettingsPageTitleFontSize); got != 24 {
		t.Fatalf("comfortable title = %v, want 24", got)
	}
	compact := ControlTheme{DensityScale: 0.9}
	if got := compact.Scaled(SettingsLabelFontSize); got != 12 {
		t.Fatalf("compact label = %v, want 12", got)
	}
}
