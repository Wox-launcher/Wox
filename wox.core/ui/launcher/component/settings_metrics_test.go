package component

import "testing"

func TestSettingsDefaultWindowFitsPluginDetails(t *testing.T) {
	if SettingsWindowWidth != 1100 || SettingsWindowHeight != 760 {
		t.Fatalf("settings window = %.0fx%.0f, want 1100x760", SettingsWindowWidth, SettingsWindowHeight)
	}
	if SettingsRailMaxWidth != 220 || SettingsPageFormMaxWidth != 700 {
		t.Fatalf("settings chrome = rail %.0f form %.0f, want 220/700", SettingsRailMaxWidth, SettingsPageFormMaxWidth)
	}
	rail := SettingsRailWidth(SettingsWindowWidth)
	page := SettingsWindowWidth - rail
	if rail != SettingsRailMaxWidth {
		t.Fatalf("default rail = %.0f, want %.0f", rail, SettingsRailMaxWidth)
	}
	if page < SettingsPageFrameWidth() || page-SettingsPageHorizontalInset*2 < SettingsPageFormMaxWidth {
		t.Fatalf("default page = %.0f, want a filled %.0f form column", page, SettingsPageFormMaxWidth)
	}
}

func TestSettingsCatalogListWidthMatchesPluginCatalog(t *testing.T) {
	if SettingsCatalogListMinWidth != 220 || SettingsCatalogListMaxWidth != 250 || SettingsCatalogDividerGutter != 21 {
		t.Fatalf("catalog column = min %.0f max %.0f gutter %.0f, want 220/250/21", SettingsCatalogListMinWidth, SettingsCatalogListMaxWidth, SettingsCatalogDividerGutter)
	}
	if got := SettingsCatalogListWidth(840); got != 250 {
		t.Fatalf("default catalog list = %.0f, want the 250 plugin column", got)
	}
	if got := SettingsCatalogListWidth(700); got != 220 {
		t.Fatalf("narrow catalog list = %.0f, want the 220 minimum", got)
	}
	if got := SettingsCatalogListWidth(800); got != 240 {
		t.Fatalf("mid catalog list = %.0f, want 30 percent of 800", got)
	}
}

func TestSettingsRailWidthStaysOnWholeUnits(t *testing.T) {
	for _, width := range []float32{900, 1000, 1200} {
		rail := SettingsRailWidth(width)
		if rail != float32(int(rail)) || rail < SettingsRailMinWidth || rail > SettingsRailMaxWidth {
			t.Fatalf("rail at %.0f = %v, want a whole unit between %.0f and %.0f", width, rail, SettingsRailMinWidth, SettingsRailMaxWidth)
		}
	}
}
