//go:build wox_ui_smoke

package query

import (
	"context"
	"fmt"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// openGeneralSettingsAndReadChoice waits for one persisted General choice and returns its visible value.
func openGeneralSettingsAndReadChoice(t *testing.T, ctx context.Context, client *automationdriver.Client, settingKey string) string {
	t.Helper()
	if err := client.OpenSettings(ctx, "/general"); err != nil {
		t.Fatalf("open General settings: %v", err)
	}
	controlID := "setting-choice-" + settingKey
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, pageFound := automationdriver.Find(snapshot, "settings.page.general")
		_, choiceFound := automationdriver.Find(snapshot, controlID)
		return pageFound && choiceFound
	})
	if err != nil {
		t.Fatalf("wait for General setting %q: %v", settingKey, err)
	}
	choice, _ := automationdriver.Find(snapshot, controlID)
	return choice.Value
}

// selectGeneralSettingChoice activates one product-defined option and waits for its persisted label.
func selectGeneralSettingChoice(t *testing.T, ctx context.Context, client *automationdriver.Client, settingKey string, optionIndex int) string {
	t.Helper()
	controlID := "setting-choice-" + settingKey
	optionID := fmt.Sprintf("setting-choice-%d", optionIndex)
	if err := client.Perform(ctx, controlID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("open General setting %q choices: %v", settingKey, err)
	}
	var optionLabel string
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		choice, choiceFound := automationdriver.Find(snapshot, optionID)
		_, menuFound := automationdriver.Find(snapshot, "setting-choice-menu")
		if choiceFound {
			optionLabel = choice.Label
		}
		return menuFound && choiceFound && optionLabel != ""
	}); err != nil {
		t.Fatalf("wait for General setting %q choice %d: %v", settingKey, optionIndex, err)
	}
	if err := client.Perform(ctx, optionID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("select General setting %q choice %d: %v", settingKey, optionIndex, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		control, found := automationdriver.Find(snapshot, controlID)
		_, menuFound := automationdriver.Find(snapshot, "setting-choice-menu")
		return found && control.Value == optionLabel && !menuFound
	}); err != nil {
		t.Fatalf("wait for General setting %q choice %d to persist: %v", settingKey, optionIndex, err)
	}
	return optionLabel
}

// restoreGeneralSettingChoice returns one shared smoke setting to the value that preceded a case.
func restoreGeneralSettingChoice(t *testing.T, client *automationdriver.Client, settingKey, previousValue string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.Hide(ctx); err != nil {
		t.Errorf("hide active window before restoring General setting %q: %v", settingKey, err)
	}
	openGeneralSettingsAndReadChoice(t, ctx, client, settingKey)
	smoke.SelectSettingChoiceByLabel(t, ctx, client, "setting-choice-"+settingKey, previousValue)
	if err := client.Hide(ctx); err != nil {
		t.Errorf("close General settings after restoring %q: %v", settingKey, err)
	}
}
