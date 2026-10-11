//go:build wox_ui_smoke

package onboarding

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"

	_ "github.com/mattn/go-sqlite3"
)

const (
	onboardingAlias       = "woxob"
	onboardingAliasQuery  = "1+1"
	onboardingAliasResult = "2"
	onboardingAliasItem   = "onboarding-migrate-item-setting:query_alias:0"
)

// Test001OnboardingWizard verifies a new data directory can finish the first-run guide and import one launcher shortcut.
// Flow: confirm onboarding is unfinished -> open the guide -> import a fixture query alias -> continue through every page -> finish.
// Evidence: the launcher opens, OnboardingFinished is saved, and the imported alias returns the calculator result.
func Test001OnboardingWizard(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		fixture := writeOnboardingFlowFixture(t)
		t.Cleanup(func() {
			_ = os.RemoveAll(fixture)
			cleanupCtx, cancel := context.WithTimeout(context.Background(), smoke.CaseTimeout)
			defer cancel()
			closeOnboardingIfOpen(t, cleanupCtx, client)
			removeOnboardingAlias(t, cleanupCtx, client)
		})

		if finished, err := readOnboardingSetting(t, "OnboardingFinished"); err == nil && finished == "true" {
			t.Fatal("onboarding is already finished")
		}
		openOnboarding(t, ctx, client)
		for _, step := range onboardingWizardSteps() {
			waitOnboardingStep(t, ctx, client, step)
			if step == "migrate" {
				importOnboardingAlias(t, ctx, client)
			}
			if step == "finish" {
				activateOnboarding(t, ctx, client, "onboarding-finish")
				break
			}
			activateOnboarding(t, ctx, client, "onboarding-next")
		}

		if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
			return state.Visible
		}); err != nil {
			t.Fatalf("wait for launcher after onboarding: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, onboardingOpen := automationdriver.Find(snapshot, "onboarding.window")
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			return !onboardingOpen && inputFound && input.Value == ""
		}); err != nil {
			t.Fatalf("wait for launcher after the guide closes: %v", err)
		}
		waitOnboardingSetting(t, ctx, "OnboardingFinished", "true")

		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, onboardingAlias)
		if !smoke.HasLauncherResultLabel(snapshot, onboardingAliasResult) {
			t.Fatalf("imported alias %q did not show calculator result %q", onboardingAlias, onboardingAliasResult)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

func onboardingWizardSteps() []string {
	steps := []string{"welcome"}
	if runtime.GOOS == "darwin" {
		steps = append(steps, "permissions")
	}
	return append(steps, "migrate", "mainHotkey", "glance", "queryHotkeys", "wpmInstall", "themeInstall", "finish")
}

func openOnboarding(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.OpenSettings(ctx, "/about"); err != nil {
		t.Fatalf("open About settings: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		button, found := automationdriver.Find(snapshot, "about-open-onboarding-button")
		return found && button.Enabled
	}); err != nil {
		t.Fatalf("wait for onboarding button: %v", err)
	}
	if err := client.Perform(ctx, "about-open-onboarding-button", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("open onboarding: %v", err)
	}
	waitOnboardingStep(t, ctx, client, "welcome")
}

func importOnboardingAlias(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		item, found := automationdriver.Find(snapshot, onboardingAliasItem)
		return found && item.Checked
	}); err != nil {
		t.Fatalf("wait for importable query alias: %v", err)
	}
	for _, id := range onboardingMigrationItemsToSkip() {
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			item, found := automationdriver.Find(snapshot, id)
			return found && item.Checked
		}); err != nil {
			t.Fatalf("wait for migration item %s: %v", id, err)
		}
		if err := client.Perform(ctx, id, woxui.AccessibilityActionToggle, ""); err != nil {
			t.Fatalf("clear migration item %s: %v", id, err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			item, found := automationdriver.Find(snapshot, id)
			return found && !item.Checked
		}); err != nil {
			t.Fatalf("wait for cleared migration item %s: %v", id, err)
		}
	}
	activateOnboarding(t, ctx, client, "onboarding-next")
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, aliasVisible := automationdriver.Find(snapshot, onboardingAliasItem)
		next, nextFound := automationdriver.Find(snapshot, "onboarding-next")
		step, stepFound := automationdriver.Find(snapshot, "onboarding-step-migrate")
		return !aliasVisible && nextFound && next.Enabled && stepFound && step.Selected
	}); err != nil {
		t.Fatalf("wait for imported query alias: %v", err)
	}
}

func onboardingMigrationItemsToSkip() []string {
	return []string{
		"onboarding-migrate-item-setting:ignore_fullscreen",
		"onboarding-migrate-item-setting:start_page",
		"onboarding-migrate-item-setting:pinyin",
		"onboarding-migrate-item-setting:input_method",
		"onboarding-migrate-item-setting:autostart",
		"onboarding-migrate-item-setting:hide_on_start",
		"onboarding-migrate-item-setting:hide_on_lost_focus",
		"onboarding-migrate-item-setting:show_tray",
		"onboarding-migrate-item-setting:auto_update",
	}
}

func activateOnboarding(t *testing.T, ctx context.Context, client *automationdriver.Client, id string) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		button, found := automationdriver.Find(snapshot, id)
		return found && button.Enabled
	}); err != nil {
		t.Fatalf("wait for %s: %v", id, err)
	}
	if err := client.Perform(ctx, id, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("activate %s: %v", id, err)
	}
}

func waitOnboardingStep(t *testing.T, ctx context.Context, client *automationdriver.Client, step string) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		node, found := automationdriver.Find(snapshot, "onboarding-step-"+step)
		return found && node.Selected
	}); err != nil {
		t.Fatalf("wait for onboarding step %s: %v", step, err)
	}
}

func writeOnboardingFlowFixture(t *testing.T) string {
	t.Helper()
	root := strings.TrimSpace(os.Getenv(util.TestFlowUserDataEnv))
	if root == "" {
		t.Fatalf("%s is not configured; run smoke through make smoke", util.TestFlowUserDataEnv)
	}
	path := filepath.Join(root, "Settings", "Settings.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create flow fixture: %v", err)
	}
	contents := `{"CustomShortcuts":[{"Key":"` + onboardingAlias + `","Value":"` + onboardingAliasQuery + `"}]}`
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write flow fixture: %v", err)
	}
	return root
}

func closeOnboardingIfOpen(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read onboarding cleanup state: %v", err)
	}
	if _, found := automationdriver.Find(snapshot, "onboarding.window"); !found {
		return
	}
	if err := client.OpenSettings(ctx, "/general"); err != nil {
		t.Fatalf("close onboarding during cleanup: %v", err)
	}
}

func removeOnboardingAlias(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.OpenSettings(ctx, "/general"); err != nil {
		t.Fatalf("open general settings to remove imported alias: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		add, found := automationdriver.Find(snapshot, "general-settings-field-0-add")
		return found && add.Enabled
	}); err != nil {
		t.Fatalf("wait for query alias table: %v", err)
	}
	for {
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read imported alias rows: %v", err)
		}
		row, found := onboardingAliasRow(snapshot)
		if !found {
			break
		}
		smoke.RemoveApplicationTableRow(t, ctx, client, "general-settings-field-0", row)
	}
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide after removing imported alias: %v", err)
	}
}

func onboardingAliasRow(snapshot woxwidget.AutomationSnapshot) (int, bool) {
	prefix := "general-settings-field-0-row-"
	const suffix = "-cell-0"
	for _, node := range snapshot.Tree.Nodes {
		if !strings.HasPrefix(node.AutomationID, prefix) || !strings.HasSuffix(node.AutomationID, suffix) || node.Value != onboardingAlias {
			continue
		}
		middle := strings.TrimSuffix(strings.TrimPrefix(node.AutomationID, prefix), suffix)
		index := 0
		for _, digit := range middle {
			if digit < '0' || digit > '9' {
				index = -1
				break
			}
			index = index*10 + int(digit-'0')
		}
		if index >= 0 && middle != "" {
			return index, true
		}
	}
	return 0, false
}

func waitOnboardingSetting(t *testing.T, ctx context.Context, key, want string) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var last string
	var lastErr error
	for {
		value, err := readOnboardingSetting(t, key)
		if err == nil && value == want {
			return
		}
		last, lastErr = value, err
		select {
		case <-ctx.Done():
			t.Fatalf("wait for %s=%q: last=%q err=%v: %v", key, want, last, lastErr, ctx.Err())
		case <-ticker.C:
		}
	}
}

func readOnboardingSetting(t *testing.T, key string) (string, error) {
	t.Helper()
	userDataDirectory := strings.TrimSpace(os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment))
	if userDataDirectory == "" {
		t.Fatalf("%s is not configured; run smoke through make smoke", automationdriver.SharedUserDataDirectoryEnvironment)
	}
	database, err := sql.Open("sqlite3", filepath.Join(userDataDirectory, "wox.db")+"?mode=ro&_busy_timeout=1000")
	if err != nil {
		return "", err
	}
	defer database.Close()
	var value string
	err = database.QueryRow("SELECT value FROM wox_settings WHERE key = ?", key).Scan(&value)
	return value, err
}
