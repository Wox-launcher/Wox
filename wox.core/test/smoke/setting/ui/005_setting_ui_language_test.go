//go:build wox_ui_smoke

package ui

import (
	"context"
	"fmt"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test005SettingUILanguage verifies that changing the UI language setting immediately localizes Settings.
// Flow: open UI settings -> select a different supported language -> observe the rebuilt Settings navigation and control.
// Evidence: the real Settings UI exposes the target-language UI and Language labels while retaining the selected language.
func Test005SettingUILanguage(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		previousLanguage := smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "LangCode")
		targetLanguage := "English"
		expectedUILabel := "UI"
		expectedLanguageLabel := "Language"
		if previousLanguage == targetLanguage {
			targetLanguage = "简体中文"
			expectedUILabel = "界面"
			expectedLanguageLabel = "语言"
		}
		t.Cleanup(func() {
			smoke.RestoreSettingChoice(t, client, "/appearance", "LangCode", previousLanguage)
		})

		smoke.SelectSettingChoiceByLabel(t, ctx, client, "setting-choice-LangCode", targetLanguage)
		// The localized labels are the only wait condition. Requiring an empty
		// Diagnostics list here instead made a stuck rebuild report a bare
		// deadline, so diagnostics stay an explicit assertion below.
		snapshot, err := client.WaitForReason(ctx, func(snapshot woxwidget.AutomationSnapshot) (bool, string) {
			uiNav, uiFound := automationdriver.Find(snapshot, "settings-nav-ui")
			languageChoice, languageFound := automationdriver.Find(snapshot, "setting-choice-LangCode")
			localized := uiFound && uiNav.Label == expectedUILabel &&
				languageFound && languageChoice.Label == expectedLanguageLabel && languageChoice.Value == targetLanguage
			if localized {
				return true, ""
			}
			return false, fmt.Sprintf("want nav %q, language label %q, language value %q; got %s",
				expectedUILabel, expectedLanguageLabel, targetLanguage,
				automationdriver.DescribeNodes(snapshot, "settings-nav-ui", "setting-choice-LangCode"))
		})
		if err != nil {
			t.Fatalf("wait for Settings to switch to %q: %v", targetLanguage, err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}
