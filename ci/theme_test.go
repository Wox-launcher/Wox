package main

import "testing"

func TestThemeLogNameUsesEnglishTranslation(t *testing.T) {
	theme := storeThemeManifest{
		Id:   "807b4689-606f-4e92-8eac-35e58c5d4f87",
		Name: "i18n:theme_name",
		I18n: map[string]map[string]string{
			"en_US": {"theme_name": "Knit"},
			"zh_CN": {"theme_name": "织"},
		},
	}
	if got := themeLogName(theme); got != "Knit" {
		t.Fatalf("themeLogName() = %q, want Knit", got)
	}
}

func TestThemeLogNameKeepsPlainName(t *testing.T) {
	theme := storeThemeManifest{Id: "saffron", Name: "Saffron"}
	if got := themeLogName(theme); got != "Saffron" {
		t.Fatalf("themeLogName() = %q, want Saffron", got)
	}
}

func TestThemeLogNameFallsBackToId(t *testing.T) {
	theme := storeThemeManifest{Id: "theme-id", Name: "i18n:theme_name"}
	if got := themeLogName(theme); got != "theme-id" {
		t.Fatalf("themeLogName() = %q, want theme-id", got)
	}
}
