//go:build wox_ui_smoke

package ui

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wox/common"
	"wox/setting"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"

	_ "github.com/mattn/go-sqlite3"
)

const autoThemeSmokeNamePrefix = "SmokeAuto-"

// Test006SettingUIAutoTheme verifies that a named Auto theme created from Installed Themes persists and can be applied.
// Flow: open Installed Themes -> create an Auto theme -> name it -> save -> apply it from the catalog.
// Evidence: the catalog marks the new Auto theme Applied, the theme document keeps auto endpoints, and ThemeId stays on that Auto document.
func Test006SettingUIAutoTheme(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		previousThemeID := persistedThemeID(t)
		name := fmt.Sprintf("%s%d", autoThemeSmokeNamePrefix, time.Now().UnixNano()%1_000_000_000)
		openInstalledThemes(t, ctx, client)

		if err := client.Perform(ctx, "theme-create-auto", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("open Auto theme editor: %v", err)
		}
		if _, err := client.WaitForReason(ctx, func(snapshot woxwidget.AutomationSnapshot) (bool, string) {
			_, nameFound := automationdriver.Find(snapshot, "theme-auto-name")
			save, saveFound := automationdriver.Find(snapshot, "theme-auto-save")
			ready := nameFound && saveFound && save.Enabled
			if ready {
				return true, ""
			}
			return false, automationdriver.DescribeNodes(snapshot, "theme-auto-name", "theme-auto-save")
		}); err != nil {
			t.Fatalf("wait for Auto theme editor: %v", err)
		}
		if err := client.Perform(ctx, "theme-auto-name", woxui.AccessibilityActionSetValue, name); err != nil {
			t.Fatalf("set Auto theme name: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			nameField, found := automationdriver.Find(snapshot, "theme-auto-name")
			return found && nameField.Value == name
		}); err != nil {
			t.Fatalf("wait for Auto theme name %q: %v", name, err)
		}
		if err := client.Perform(ctx, "theme-auto-save", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("save Auto theme: %v", err)
		}

		themeID := waitForSavedAutoTheme(t, ctx, client, name)
		t.Cleanup(func() {
			cleanupCreatedAutoTheme(t, client, name, themeID, previousThemeID)
		})
		waitForAutoThemeDocument(t, ctx, themeID, name)

		if err := client.Perform(ctx, "theme-apply", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("apply Auto theme: %v", err)
		}
		snapshot, err := client.WaitForReason(ctx, func(snapshot woxwidget.AutomationSnapshot) (bool, string) {
			apply, found := automationdriver.Find(snapshot, "theme-apply")
			applied := found && !apply.Enabled && !strings.HasSuffix(apply.Label, "…")
			if applied {
				return true, ""
			}
			return false, automationdriver.DescribeNodes(snapshot, "theme-apply")
		})
		if err != nil {
			t.Fatalf("wait for Auto theme to become Applied: %v", err)
		}
		waitForPersistedThemeID(t, ctx, themeID)
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// openInstalledThemes opens the Installed Themes catalog and waits until create and list rows are actionable.
func openInstalledThemes(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.OpenSettings(ctx, "/themes"); err != nil {
		t.Fatalf("open Installed Themes: %v", err)
	}
	if _, err := client.WaitForReason(ctx, func(snapshot woxwidget.AutomationSnapshot) (bool, string) {
		_, pageFound := automationdriver.Find(snapshot, "settings.page.theme")
		create, createFound := automationdriver.Find(snapshot, "theme-create-auto")
		_, listFound := automationdriver.FindByAutomationIDPrefix(snapshot, "theme-list-")
		ready := pageFound && createFound && create.Enabled && listFound
		if ready {
			return true, ""
		}
		return false, automationdriver.DescribeNodes(snapshot, "settings.page.theme", "theme-create-auto")
	}); err != nil {
		t.Fatalf("wait for Installed Themes: %v", err)
	}
}

// waitForSavedAutoTheme waits until the named Auto theme is selected in the catalog after save.
func waitForSavedAutoTheme(t *testing.T, ctx context.Context, client *automationdriver.Client, name string) string {
	t.Helper()
	var themeID string
	if _, err := client.WaitForReason(ctx, func(snapshot woxwidget.AutomationSnapshot) (bool, string) {
		_, editorFound := automationdriver.Find(snapshot, "theme-auto-save")
		item, itemFound := findThemeListItem(snapshot, name)
		apply, applyFound := automationdriver.Find(snapshot, "theme-apply")
		if !editorFound && itemFound && item.Selected && applyFound && apply.Enabled {
			themeID = strings.TrimPrefix(item.AutomationID, "theme-list-")
			return true, ""
		}
		return false, fmt.Sprintf("editor=%t %s %s", editorFound, describeThemeListItem(snapshot, name), automationdriver.DescribeNodes(snapshot, "theme-apply"))
	}); err != nil {
		t.Fatalf("wait for saved Auto theme %q: %v", name, err)
	}
	if themeID == "" {
		t.Fatalf("saved Auto theme %q has an empty id", name)
	}
	return themeID
}

// waitForAutoThemeDocument polls the persisted Auto theme file for name and default endpoints.
func waitForAutoThemeDocument(t *testing.T, ctx context.Context, themeID, name string) {
	t.Helper()
	path := autoThemeDocumentPath(t, themeID)
	smoke.WaitForFile(t, ctx, path, func(data []byte) bool {
		var theme common.Theme
		if err := json.Unmarshal(data, &theme); err != nil {
			return false
		}
		return theme.ThemeId == themeID && theme.ThemeName == name && theme.IsAutoAppearance &&
			theme.LightThemeId == setting.DefaultLightThemeId && theme.DarkThemeId == setting.DefaultDarkThemeId
	})
}

// cleanupCreatedAutoTheme uninstalls the smoke Auto theme and restores the previous ThemeId.
func cleanupCreatedAutoTheme(t *testing.T, client *automationdriver.Client, name, themeID, previousThemeID string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.Hide(ctx); err != nil {
		t.Errorf("hide windows before uninstalling Auto theme: %v", err)
	}
	openInstalledThemes(t, ctx, client)
	if err := client.Perform(ctx, "theme-search", woxui.AccessibilityActionSetValue, name); err != nil {
		t.Errorf("search for Auto theme %q during cleanup: %v", name, err)
		return
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		item, found := findThemeListItem(snapshot, name)
		return found && item.AutomationID == "theme-list-"+themeID
	}); err != nil {
		t.Errorf("wait for Auto theme %q during cleanup: %v", name, err)
		return
	}
	if err := client.Perform(ctx, "theme-list-"+themeID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Errorf("select Auto theme %q during cleanup: %v", name, err)
		return
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		uninstall, found := automationdriver.Find(snapshot, "theme-uninstall")
		return found && uninstall.Enabled
	}); err != nil {
		t.Errorf("wait for uninstall on Auto theme %q: %v", name, err)
		return
	}
	if err := client.Perform(ctx, "theme-uninstall", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Errorf("arm uninstall for Auto theme %q: %v", name, err)
		return
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		uninstall, found := automationdriver.Find(snapshot, "theme-uninstall")
		return found && strings.Contains(uninstall.Label, "Confirm")
	}); err != nil {
		t.Errorf("wait for uninstall confirmation on Auto theme %q: %v", name, err)
		return
	}
	if err := client.Perform(ctx, "theme-uninstall", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Errorf("uninstall Auto theme %q: %v", name, err)
		return
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := findThemeListItem(snapshot, name)
		return !found
	}); err != nil {
		t.Errorf("wait for Auto theme %q to leave the catalog: %v", name, err)
	}
	if previousThemeID != "" && previousThemeID != setting.DefaultThemeId && persistedThemeID(t) != previousThemeID {
		restorePreviousTheme(t, ctx, client, previousThemeID)
	}
	if err := client.Hide(ctx); err != nil {
		t.Errorf("close settings after uninstalling Auto theme: %v", err)
	}
}

// restorePreviousTheme applies the ThemeId that was active before the smoke Auto theme.
func restorePreviousTheme(t *testing.T, ctx context.Context, client *automationdriver.Client, themeID string) {
	t.Helper()
	if err := client.Perform(ctx, "theme-search", woxui.AccessibilityActionSetValue, ""); err != nil {
		t.Errorf("clear theme search before restoring %q: %v", themeID, err)
		return
	}
	rowID := "theme-list-" + themeID
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, rowID)
		return found
	}); err != nil {
		t.Errorf("wait for previous theme %q: %v", themeID, err)
		return
	}
	if err := client.Perform(ctx, rowID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Errorf("select previous theme %q: %v", themeID, err)
		return
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		apply, found := automationdriver.Find(snapshot, "theme-apply")
		return found && apply.Enabled
	}); err != nil {
		t.Errorf("wait to apply previous theme %q: %v", themeID, err)
		return
	}
	if err := client.Perform(ctx, "theme-apply", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Errorf("apply previous theme %q: %v", themeID, err)
		return
	}
	waitForPersistedThemeID(t, ctx, themeID)
}

func findThemeListItem(snapshot woxwidget.AutomationSnapshot, name string) (woxui.AccessibilityNode, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "theme-list-") && node.Label == name {
			return node, true
		}
	}
	return woxui.AccessibilityNode{}, false
}

func describeThemeListItem(snapshot woxwidget.AutomationSnapshot, name string) string {
	item, found := findThemeListItem(snapshot, name)
	if !found {
		return fmt.Sprintf("theme %q missing", name)
	}
	return fmt.Sprintf("theme %q id=%q selected=%t", name, item.AutomationID, item.Selected)
}

func autoThemeDocumentPath(t *testing.T, themeID string) string {
	t.Helper()
	userDataDirectory := strings.TrimSpace(os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment))
	if userDataDirectory == "" {
		t.Fatalf("%s is not configured", automationdriver.SharedUserDataDirectoryEnvironment)
	}
	return filepath.Join(userDataDirectory, "themes", themeID+".json")
}

func waitForPersistedThemeID(t *testing.T, ctx context.Context, want string) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var last string
	for {
		last = persistedThemeID(t)
		if last == want {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for ThemeId %q: last=%q: %v", want, last, ctx.Err())
		case <-ticker.C:
		}
	}
}

func persistedThemeID(t *testing.T) string {
	t.Helper()
	userDataDirectory := strings.TrimSpace(os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment))
	if userDataDirectory == "" {
		t.Fatalf("%s is not configured", automationdriver.SharedUserDataDirectoryEnvironment)
	}
	db, err := sql.Open("sqlite3", filepath.Join(userDataDirectory, "wox.db")+"?mode=ro&_busy_timeout=5000")
	if err != nil {
		t.Fatalf("open smoke database: %v", err)
	}
	defer db.Close()
	var value string
	if err := db.QueryRow("SELECT value FROM wox_settings WHERE key = ?", "ThemeId").Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			return ""
		}
		t.Fatalf("read ThemeId: %v", err)
	}
	return value
}
