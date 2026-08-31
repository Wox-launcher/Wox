//go:build wox_ui_smoke

package general

import (
	"context"
	"fmt"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test004SettingGeneralLanguage verifies that changing the General language setting immediately localizes Settings.
// Flow: open General settings -> select a different supported language -> observe the rebuilt Settings navigation and control.
// Evidence: the real Settings UI exposes the target-language General and Language labels while retaining the selected language.
func Test004SettingGeneralLanguage(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		previousLanguage := smoke.OpenGeneralSettingsAndReadChoice(t, ctx, client, "LangCode")
		targetLanguage := "English"
		expectedGeneralLabel := "General"
		expectedLanguageLabel := "Language"
		if previousLanguage == targetLanguage {
			targetLanguage = "简体中文"
			expectedGeneralLabel = "通用"
			expectedLanguageLabel = "语言"
		}
		t.Cleanup(func() {
			smoke.RestoreGeneralSettingChoice(t, client, "LangCode", previousLanguage)
		})

		smoke.SelectSettingChoiceByLabel(t, ctx, client, "setting-choice-LangCode", targetLanguage)
		// The localized labels are the only wait condition. Requiring an empty
		// Diagnostics list here instead made a stuck rebuild report a bare
		// deadline, so diagnostics stay an explicit assertion below.
		snapshot, err := client.WaitForReason(ctx, func(snapshot woxwidget.AutomationSnapshot) (bool, string) {
			generalNav, generalFound := automationdriver.Find(snapshot, "settings-nav-general")
			languageChoice, languageFound := automationdriver.Find(snapshot, "setting-choice-LangCode")
			localized := generalFound && generalNav.Label == expectedGeneralLabel &&
				languageFound && languageChoice.Label == expectedLanguageLabel && languageChoice.Value == targetLanguage
			if localized {
				return true, ""
			}
			return false, fmt.Sprintf("want nav %q, language label %q, language value %q; got %s",
				expectedGeneralLabel, expectedLanguageLabel, targetLanguage,
				automationdriver.DescribeNodes(snapshot, "settings-nav-general", "setting-choice-LangCode"))
		})
		if err != nil {
			t.Fatalf("wait for Settings to switch to %q: %v", targetLanguage, err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
