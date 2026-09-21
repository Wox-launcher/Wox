package common

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"wox/i18n"
)

// TestThemeMetadataI18n covers language changes, fallback and lossless authored metadata.
func TestThemeMetadataI18n(t *testing.T) {
	ctx := context.Background()
	manager := i18n.GetI18nManager()
	original := manager.GetCurrentLangCode()
	t.Cleanup(func() { _ = manager.UpdateLang(ctx, original) })
	input := strings.Replace(minimalV2Theme, `"ThemeName":"Test"`, `"ThemeName":"i18n:name","Description":"i18n:description","I18n":{"en_US":{"name":"Ming","description":"Imperial theme"},"zh_CN":{"name":"明"}}`, 1)
	var theme Theme
	if err := json.Unmarshal([]byte(input), &theme); err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"windows", "macos", "linux"} {
		resolved, err := theme.ResolveForTarget(platform, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, lang := range []i18n.LangCode{"zh_CN", "en_US", "ja_JP", "zh_CN"} {
			if err := manager.UpdateLang(ctx, lang); err != nil {
				t.Fatal(err)
			}
			want := "Ming"
			if lang == "zh_CN" {
				want = "明"
			}
			if resolved.GetName(ctx) != want || resolved.GetDescription(ctx) != "Imperial theme" {
				t.Fatalf("%s/%s: %s / %s", platform, lang, resolved.GetName(ctx), resolved.GetDescription(ctx))
			}
		}
		data, err := json.Marshal(resolved)
		if err != nil {
			t.Fatal(err)
		}
		var saved Theme
		if err := json.Unmarshal(data, &saved); err != nil {
			t.Fatal(err)
		}
		if saved.ThemeName != "i18n:name" || saved.Description != "i18n:description" || saved.I18n["zh_CN"]["name"] != "明" {
			t.Fatal("saving replaced translation keys or lost translations")
		}
	}
	if (Theme{ThemeName: "Literal"}).GetName(ctx) != "Literal" || (Theme{ThemeName: "i18n:missing_theme_key"}).GetName(ctx) != "i18n:missing_theme_key" {
		t.Fatal("literal or missing-key fallback changed")
	}
}
