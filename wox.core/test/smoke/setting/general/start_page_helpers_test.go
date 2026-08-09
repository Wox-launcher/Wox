//go:build wox_ui_smoke

package general

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"

	_ "github.com/mattn/go-sqlite3"
)

const (
	converterPluginID = "a48dc5f0-dab9-4112-b883-b68129d6782b"
	converterMRUQuery = "1 m to cm"
)

// configureStartPage selects fresh launch behavior and one Start Page option, restoring both settings after the case.
func configureStartPage(t *testing.T, ctx context.Context, client *automationdriver.Client, startPageOption int) {
	t.Helper()
	previousLaunchMode := smoke.OpenGeneralSettingsAndReadChoice(t, ctx, client, "LaunchMode")
	freshMode := smoke.SelectSettingChoiceByIndex(t, ctx, client, "LaunchMode", 0)
	if previousLaunchMode != freshMode {
		t.Cleanup(func() { smoke.RestoreGeneralSettingChoice(t, client, "LaunchMode", previousLaunchMode) })
	}

	previousStartPage := smoke.OpenGeneralSettingsAndReadChoice(t, ctx, client, "StartPage")
	selectedStartPage := smoke.SelectSettingChoiceByIndex(t, ctx, client, "StartPage", startPageOption)
	if previousStartPage != selectedStartPage {
		t.Cleanup(func() { smoke.RestoreGeneralSettingChoice(t, client, "StartPage", previousStartPage) })
	}
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close General settings after configuring Start Page: %v", err)
	}
}

// seedConverterMRU executes one deterministic conversion enough times to cross the product's MRU eligibility threshold.
func seedConverterMRU(t *testing.T, ctx context.Context, client *automationdriver.Client) string {
	t.Helper()
	smoke.PreserveClipboard(t)
	resultLabel := ""
	for useCount := 0; useCount < 3; useCount++ {
		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, converterMRUQuery)
		resultID := ""
		for _, node := range snapshot.Tree.Nodes {
			if !strings.HasPrefix(node.AutomationID, "launcher.result.") {
				continue
			}
			if resultLabel == "" && strings.Contains(node.Label, "100") {
				resultLabel = node.Label
			}
			if resultLabel != "" && node.Label == resultLabel {
				resultID = node.AutomationID
				break
			}
		}
		if resultID == "" {
			t.Fatalf("find deterministic Converter result on use %d: label %q", useCount+1, resultLabel)
		}
		if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("execute Converter result on use %d: %v", useCount+1, err)
		}
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("hide launcher after Converter use %d: %v", useCount+1, err)
		}
	}
	waitForConverterMRUUseCount(t, ctx, resultLabel, 3)
	t.Cleanup(func() { deleteConverterMRUSeed(t, resultLabel) })
	return converterMRUQuery + ": " + resultLabel
}

// waitForConverterMRUUseCount polls the real isolated database until the asynchronous action record is durable.
func waitForConverterMRUUseCount(t *testing.T, ctx context.Context, resultLabel string, minimum int) {
	t.Helper()
	db := openSmokeDatabase(t)
	defer db.Close()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var lastErr error
	for {
		var useCount int
		lastErr = db.QueryRowContext(ctx, "SELECT use_count FROM mru_records WHERE plugin_id = ? AND title = ?", converterPluginID, resultLabel).Scan(&useCount)
		if lastErr == nil && useCount >= minimum {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for Converter MRU use count %d: last error %v", minimum, lastErr)
		case <-ticker.C:
		}
	}
}

// deleteConverterMRUSeed removes only the deterministic record created by the current smoke case.
func deleteConverterMRUSeed(t *testing.T, resultLabel string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := openSmokeDatabase(t)
	defer db.Close()
	if _, err := db.ExecContext(ctx, "DELETE FROM mru_records WHERE plugin_id = ? AND title = ?", converterPluginID, resultLabel); err != nil {
		t.Errorf("delete Converter MRU seed %q: %v", resultLabel, err)
	}
}

// openSmokeDatabase opens the suite-owned database without creating another Wox process.
func openSmokeDatabase(t *testing.T) *sql.DB {
	t.Helper()
	userDataDirectory := strings.TrimSpace(os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment))
	if userDataDirectory == "" {
		t.Fatalf("%s is not configured", automationdriver.SharedUserDataDirectoryEnvironment)
	}
	db, err := sql.Open("sqlite3", filepath.Join(userDataDirectory, "wox.db")+"?_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open shared smoke database: %v", err)
	}
	return db
}

// hasLauncherResults reports whether the current generation exposes any result row.
func hasLauncherResults(snapshot woxwidget.AutomationSnapshot) bool {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") {
			return true
		}
	}
	return false
}
