package smoke

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/clipboard"
)

// OpenGeneralSettingsAndReadChoice opens General settings and returns one persisted choice label.
func OpenGeneralSettingsAndReadChoice(t *testing.T, ctx context.Context, client *automationdriver.Client, settingKey string) string {
	t.Helper()
	return OpenSettingsAndReadChoice(t, ctx, client, "/general", settingKey)
}

// OpenSettingsAndReadChoice opens a Settings section and returns one persisted choice label.
func OpenSettingsAndReadChoice(t *testing.T, ctx context.Context, client *automationdriver.Client, path, settingKey string) string {
	t.Helper()
	if err := client.OpenSettings(ctx, path); err != nil {
		t.Fatalf("open settings %q: %v", path, err)
	}
	controlID := "setting-choice-" + settingKey
	pageID := "settings.page." + strings.TrimPrefix(path, "/")
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, pageFound := automationdriver.Find(snapshot, pageID)
		_, choiceFound := automationdriver.Find(snapshot, controlID)
		return pageFound && choiceFound
	})
	if err != nil {
		t.Fatalf("wait for setting %q on %q: %v", settingKey, path, err)
	}
	choice, _ := automationdriver.Find(snapshot, controlID)
	return choice.Value
}

// OpenGeneralSettingsAndReadSwitch opens General settings and returns one persisted boolean value.
func OpenGeneralSettingsAndReadSwitch(t *testing.T, ctx context.Context, client *automationdriver.Client, settingKey string) bool {
	t.Helper()
	return OpenSettingsAndReadSwitch(t, ctx, client, "/general", settingKey)
}

// OpenSettingsAndReadSwitch opens a Settings section and returns one persisted switch value.
func OpenSettingsAndReadSwitch(t *testing.T, ctx context.Context, client *automationdriver.Client, path, settingKey string) bool {
	t.Helper()
	if err := client.OpenSettings(ctx, path); err != nil {
		t.Fatalf("open settings %q: %v", path, err)
	}
	controlID := "setting-switch-" + settingKey
	pageID := "settings.page." + strings.TrimPrefix(path, "/")
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, pageFound := automationdriver.Find(snapshot, pageID)
		_, switchFound := automationdriver.Find(snapshot, controlID)
		return pageFound && switchFound
	})
	if err != nil {
		t.Fatalf("wait for switch %q on %q: %v", settingKey, path, err)
	}
	control, _ := automationdriver.Find(snapshot, controlID)
	return control.Checked
}

// SetSettingSwitch toggles one visible boolean setting only when its persisted value differs.
func SetSettingSwitch(t *testing.T, ctx context.Context, client *automationdriver.Client, settingKey string, expected bool) {
	t.Helper()
	controlID := "setting-switch-" + settingKey
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read setting switch %q: %v", settingKey, err)
	}
	control, found := automationdriver.Find(snapshot, controlID)
	if !found {
		t.Fatalf("setting switch %q was not found", settingKey)
	}
	if control.Checked != expected {
		if err := client.Perform(ctx, controlID, woxui.AccessibilityActionToggle, ""); err != nil {
			t.Fatalf("toggle setting switch %q: %v", settingKey, err)
		}
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		control, found := automationdriver.Find(snapshot, controlID)
		return found && control.Checked == expected
	}); err != nil {
		t.Fatalf("wait for setting switch %q to become %t: %v", settingKey, expected, err)
	}
}

// SelectSettingChoiceByIndex activates one product-defined option and waits for persistence.
func SelectSettingChoiceByIndex(t *testing.T, ctx context.Context, client *automationdriver.Client, settingKey string, optionIndex int) string {
	t.Helper()
	controlID := "setting-choice-" + settingKey
	optionID := fmt.Sprintf("setting-choice-%d", optionIndex)
	if err := client.Perform(ctx, controlID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("open setting %q choices: %v", settingKey, err)
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
		t.Fatalf("wait for setting %q choice %d: %v", settingKey, optionIndex, err)
	}
	if err := client.Perform(ctx, optionID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("select setting %q choice %d: %v", settingKey, optionIndex, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		control, found := automationdriver.Find(snapshot, controlID)
		_, menuFound := automationdriver.Find(snapshot, "setting-choice-menu")
		return found && control.Value == optionLabel && !menuFound
	}); err != nil {
		t.Fatalf("wait for setting %q choice %d to persist: %v", settingKey, optionIndex, err)
	}
	return optionLabel
}

// RestoreGeneralSettingChoice returns one shared General setting to its previous value.
func RestoreGeneralSettingChoice(t *testing.T, client *automationdriver.Client, settingKey, previousValue string) {
	t.Helper()
	RestoreSettingChoice(t, client, "/general", settingKey, previousValue)
}

// RestoreSettingChoice restores one shared Settings choice through its owning section.
func RestoreSettingChoice(t *testing.T, client *automationdriver.Client, path, settingKey, previousValue string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.Hide(ctx); err != nil {
		t.Errorf("hide active window before restoring setting %q: %v", settingKey, err)
	}
	OpenSettingsAndReadChoice(t, ctx, client, path, settingKey)
	SelectSettingChoiceByLabel(t, ctx, client, "setting-choice-"+settingKey, previousValue)
	if err := client.Hide(ctx); err != nil {
		t.Errorf("close settings after restoring %q: %v", settingKey, err)
	}
}

// RestoreGeneralSettingSwitch returns one shared General switch to its previous value.
func RestoreGeneralSettingSwitch(t *testing.T, client *automationdriver.Client, settingKey string, previousValue bool) {
	t.Helper()
	RestoreSettingSwitch(t, client, "/general", settingKey, previousValue)
}

// RestoreSettingSwitch restores one shared Settings switch through its owning section.
func RestoreSettingSwitch(t *testing.T, client *automationdriver.Client, path, settingKey string, previousValue bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := client.Hide(ctx); err != nil {
		t.Errorf("hide active window before restoring switch %q: %v", settingKey, err)
	}
	OpenSettingsAndReadSwitch(t, ctx, client, path, settingKey)
	SetSettingSwitch(t, ctx, client, settingKey, previousValue)
	if err := client.Hide(ctx); err != nil {
		t.Errorf("close settings after restoring switch %q: %v", settingKey, err)
	}
}

// PreserveClipboard restores the platform clipboard after a smoke case changes it.
func PreserveClipboard(t *testing.T) {
	t.Helper()
	previousClipboard, err := clipboard.Read()
	if err != nil && !errors.Is(err, clipboard.NoDataErr()) {
		t.Fatalf("read clipboard before smoke case: %v", err)
	}
	t.Cleanup(func() {
		if previousClipboard != nil {
			if restoreErr := clipboard.Write(previousClipboard); restoreErr != nil {
				t.Errorf("restore clipboard after smoke case: %v", restoreErr)
			}
			return
		}
		if restoreErr := clipboard.WriteText(""); restoreErr != nil {
			t.Errorf("clear clipboard after smoke case: %v", restoreErr)
		}
	})
}

// HasLauncherResultLabel reports whether the current launcher generation exposes one matching result.
func HasLauncherResultLabel(snapshot woxwidget.AutomationSnapshot, label string) bool {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && node.Label == label {
			return true
		}
	}
	return false
}

// FindLauncherResult returns the current dynamic result ID for an exact visible label.
func FindLauncherResult(snapshot woxwidget.AutomationSnapshot, label string) (string, bool) {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.TrimSpace(node.Label) == label {
			return node.AutomationID, true
		}
	}
	return "", false
}

// SelectLauncherResult moves native keyboard selection to a current dynamic result ID.
func SelectLauncherResult(t *testing.T, ctx context.Context, client *automationdriver.Client, resultID string) woxwidget.AutomationSnapshot {
	t.Helper()
	return selectLauncherResult(t, ctx, client, resultID, func(node woxui.AccessibilityNode) bool {
		return node.AutomationID == resultID
	})
}

// SelectLauncherResultLabelPrefix moves selection to the visible result whose label starts with prefix.
// Plugins such as Clipboard mint a new result ID when the watcher republishes, so ID-based selection is racy.
func SelectLauncherResultLabelPrefix(t *testing.T, ctx context.Context, client *automationdriver.Client, prefix string) woxwidget.AutomationSnapshot {
	t.Helper()
	return selectLauncherResult(t, ctx, client, prefix, func(node woxui.AccessibilityNode) bool {
		return strings.HasPrefix(node.Label, prefix)
	})
}

// selectLauncherResult moves keyboard selection to the current result matching match.
func selectLauncherResult(t *testing.T, ctx context.Context, client *automationdriver.Client, description string, match func(woxui.AccessibilityNode) bool) woxwidget.AutomationSnapshot {
	t.Helper()
	resultCount, selectedIndex, targetIndex := 0, -1, -1
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		resultCount, selectedIndex, targetIndex = launcherResultMatchIndexes(snapshot, match)
		return selectedIndex >= 0 && targetIndex >= 0
	})
	if err != nil {
		t.Fatalf("wait for launcher result %q before selecting: %v", description, err)
	}
	for range resultCount {
		if targetIndex == selectedIndex {
			return snapshot
		}
		key := woxui.KeyArrowDown
		if targetIndex < selectedIndex {
			key = woxui.KeyArrowUp
		}
		if err := client.PressKey(ctx, key, 0); err != nil {
			t.Fatalf("navigate to launcher result %q: %v", description, err)
		}
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, selectedIndex, targetIndex = launcherResultMatchIndexes(snapshot, match)
			return selectedIndex >= 0 && targetIndex >= 0
		})
		if err != nil {
			t.Fatalf("wait for launcher result %q while selecting: %v", description, err)
		}
	}
	t.Fatalf("launcher result %q was not selected after keyboard navigation", description)
	return woxwidget.AutomationSnapshot{}
}

// launcherResultMatchIndexes returns visible result count and the selected/target indexes for match.
func launcherResultMatchIndexes(snapshot woxwidget.AutomationSnapshot, match func(woxui.AccessibilityNode) bool) (resultCount, selectedIndex, targetIndex int) {
	selectedIndex, targetIndex = -1, -1
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") {
			if node.Selected {
				selectedIndex = resultCount
			}
			if match(node) {
				targetIndex = resultCount
			}
			resultCount++
		}
	}
	return resultCount, selectedIndex, targetIndex
}

// OpenResultActionPanel opens the action panel through its platform shortcut and waits for the focused filter.
func OpenResultActionPanel(t *testing.T, ctx context.Context, client *automationdriver.Client) woxwidget.AutomationSnapshot {
	t.Helper()
	modifier := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		modifier = woxui.KeyModifierMeta
	}
	if err := client.PressKey(ctx, woxui.Key("j"), modifier); err != nil {
		t.Fatalf("open launcher result actions: %v", err)
	}
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		input, found := automationdriver.Find(snapshot, "action-search")
		return found && input.Focused
	})
	if err != nil {
		t.Fatalf("wait for launcher action panel: %v", err)
	}
	return snapshot
}

// ActivateSelectedResultAction opens the action panel and invokes the current action matching a stable prefix.
func ActivateSelectedResultAction(t *testing.T, ctx context.Context, client *automationdriver.Client, actionPrefix string) {
	t.Helper()
	OpenResultActionPanel(t, ctx, client)
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var actionID string
	if _, err := client.WaitFor(waitCtx, func(snapshot woxwidget.AutomationSnapshot) bool {
		node, found := automationdriver.FindByAutomationIDPrefix(snapshot, actionPrefix)
		if found {
			actionID = node.AutomationID
		}
		return found
	}); err != nil {
		t.Fatalf("wait for launcher result action %q: %v", actionPrefix, err)
	}
	if err := client.Perform(ctx, actionID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("activate launcher result action %q: %v", actionPrefix, err)
	}
}

// WaitForResultActionsClosed waits until the action panel has left the semantic tree.
func WaitForResultActionsClosed(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "action-result-") {
				return false
			}
		}
		return true
	}); err != nil {
		t.Fatalf("wait for launcher result actions to close: %v", err)
	}
}

// OpenInstalledPluginSettings opens one installed plugin through the shared settings route.
func OpenInstalledPluginSettings(t *testing.T, ctx context.Context, client *automationdriver.Client, pluginID string) {
	t.Helper()
	if err := client.OpenSettings(ctx, "/plugins"); err != nil {
		t.Fatalf("open plugin settings: %v", err)
	}
	selectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if _, err := client.WaitFor(selectCtx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, "plugin-search")
		return found
	}); err != nil {
		t.Fatalf("wait for installed plugin search: %v", err)
	}
	// The catalog uses a lazy list, so filter by ID before waiting for a row that may be off-screen.
	if err := client.Perform(selectCtx, "plugin-search", woxui.AccessibilityActionSetValue, pluginID); err != nil {
		t.Fatalf("filter installed plugin %q: %v", pluginID, err)
	}
	listID := "plugin-list-" + pluginID
	if _, err := client.WaitFor(selectCtx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, listID)
		return found
	}); err != nil {
		t.Fatalf("wait for installed plugin %q: %v", pluginID, err)
	}
	if err := client.Perform(selectCtx, listID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("select installed plugin %q: %v", pluginID, err)
	}
}

// WaitForApplicationCatalog waits until the shared application picker has a complete platform catalog.
func WaitForApplicationCatalog(t *testing.T, ctx context.Context, path string) {
	t.Helper()
	WaitForFile(t, ctx, path, func(data []byte) bool {
		logs := string(data)
		return strings.Contains(logs, " indexed ") && strings.Contains(logs, " apps, cost ")
	})
}

// ApplicationTableRowCount returns the number of persisted rows in one inline application table.
func ApplicationTableRowCount(t *testing.T, ctx context.Context, client *automationdriver.Client, fieldID string) int {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read application table %q rows: %v", fieldID, err)
	}
	count := 0
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, fieldID+"-row-") && strings.HasSuffix(node.AutomationID, "-delete") {
			count++
		}
	}
	return count
}

// AddApplicationTableRow selects one indexed application through the shared picker and waits for persistence.
func AddApplicationTableRow(t *testing.T, ctx context.Context, client *automationdriver.Client, fieldID, query string) int {
	t.Helper()
	rowIndex := ApplicationTableRowCount(t, ctx, client, fieldID)
	if err := client.Perform(ctx, fieldID+"-add", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("add application to %q: %v", fieldID, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, fieldFound := automationdriver.Find(snapshot, "form-table-row-field-0")
		_, saveFound := automationdriver.Find(snapshot, "form-table-row-save")
		return fieldFound && saveFound
	}); err != nil {
		t.Fatalf("wait for application row editor: %v", err)
	}
	if err := client.Perform(ctx, "form-table-row-field-0", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("open application picker: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, dialogFound := automationdriver.Find(snapshot, "form-table-app-dialog")
		_, searchFound := automationdriver.Find(snapshot, "form-table-app-search")
		return dialogFound && searchFound
	}); err != nil {
		t.Fatalf("wait for application picker: %v", err)
	}
	if err := client.Perform(ctx, "form-table-app-search", woxui.AccessibilityActionSetValue, query); err != nil {
		t.Fatalf("search applications for %q: %v", query, err)
	}

	candidateID := ""
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		candidateID = ""
		count := 0
		for _, node := range snapshot.Tree.Nodes {
			if node.Role == woxui.AccessibilityRoleRadioButton && strings.HasPrefix(node.AutomationID, "form-table-app-") {
				candidateID = node.AutomationID
				count++
			}
		}
		return count == 1
	}); err != nil {
		t.Fatalf("wait for one application candidate for %q: %v", query, err)
	}
	if err := client.Perform(ctx, candidateID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("select application %q: %v", query, err)
	}
	if err := client.Perform(ctx, "form-table-app-confirm", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("confirm application %q: %v", query, err)
	}
	if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("save application %q: %v", query, err)
	}

	rowDeleteID := fieldID + "-row-" + strconv.Itoa(rowIndex) + "-delete"
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, rowFound := automationdriver.Find(snapshot, rowDeleteID)
		add, addFound := automationdriver.Find(snapshot, fieldID+"-add")
		return rowFound && addFound && add.Enabled
	}); err != nil {
		t.Fatalf("wait for application %q to persist: %v", query, err)
	}
	return rowIndex
}

// RemoveApplicationTableRow deletes one row and waits until its table is ready again.
func RemoveApplicationTableRow(t *testing.T, ctx context.Context, client *automationdriver.Client, fieldID string, rowIndex int) {
	t.Helper()
	rowDeleteID := fieldID + "-row-" + strconv.Itoa(rowIndex) + "-delete"
	if err := client.Perform(ctx, rowDeleteID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("delete application row %d from %q: %v", rowIndex, fieldID, err)
	}
	if err := client.Perform(ctx, "form-table-delete-confirm", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("confirm application row %d deletion: %v", rowIndex, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, rowFound := automationdriver.Find(snapshot, rowDeleteID)
		add, addFound := automationdriver.Find(snapshot, fieldID+"-add")
		return !rowFound && addFound && add.Enabled
	}); err != nil {
		t.Fatalf("wait for application row %d removal: %v", rowIndex, err)
	}
}

// SelectSettingChoiceByLabel chooses one shared dropdown option and waits for its committed value.
func SelectSettingChoiceByLabel(t *testing.T, ctx context.Context, client *automationdriver.Client, controlID, expected string) {
	t.Helper()
	if err := client.Perform(ctx, controlID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("open setting choices for %q: %v", controlID, err)
	}
	var choiceID string
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "setting-choice-") && node.Label == expected {
				choiceID = node.AutomationID
				return true
			}
		}
		return false
	}); err != nil {
		t.Fatalf("wait for setting choice %q: %v", expected, err)
	}
	if err := client.Perform(ctx, choiceID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("select setting choice %q: %v", expected, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		control, found := automationdriver.Find(snapshot, controlID)
		_, menuFound := automationdriver.Find(snapshot, "setting-choice-menu")
		return found && control.Value == expected && !menuFound
	}); err != nil {
		t.Fatalf("wait for setting choice %q to commit: %v", expected, err)
	}
}

// SetLauncherQueryAndWaitComplete changes the query and waits for the matching result generation.
func SetLauncherQueryAndWaitComplete(t *testing.T, ctx context.Context, client *automationdriver.Client, query string) woxwidget.AutomationSnapshot {
	t.Helper()
	if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
		t.Fatalf("set launcher query %q: %v", query, err)
	}
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		if !resultsFound || results.Value != "complete" {
			return false
		}
		input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
		// Chat/preview-only modes hide the query box after the result lands.
		return !inputFound || input.Value == query
	})
	if err != nil {
		t.Fatalf("wait for launcher query %q: %v", query, err)
	}
	return snapshot
}

// ReplaceLauncherQuery clears retained results before submitting a fresh query generation.
func ReplaceLauncherQuery(t *testing.T, ctx context.Context, client *automationdriver.Client, query string) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read launcher query before replacing it: %v", err)
	}
	current, found := automationdriver.Find(snapshot, "launcher.query.input")
	if found && current.Value != "" {
		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, ""); err != nil {
			t.Fatalf("clear retained launcher query %q: %v", current.Value, err)
		}
		snapshot, err = client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			if !inputFound || input.Value != "" {
				return false
			}
			for _, node := range snapshot.Tree.Nodes {
				if strings.HasPrefix(node.AutomationID, "launcher.result.") {
					return false
				}
			}
			return true
		})
		if err != nil {
			t.Fatalf("wait for retained launcher query %q to clear: %v", current.Value, err)
		}
	}
	if query == "" {
		return snapshot
	}
	return SetLauncherQueryAndWaitComplete(t, ctx, client, query)
}

// WaitForFile polls one real artifact independently of UI generation changes.
func WaitForFile(t *testing.T, ctx context.Context, path string, matches func([]byte) bool) []byte {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		data, err := os.ReadFile(path)
		if err == nil && matches(data) {
			return data
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read artifact %q: %v", path, err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for artifact %q: %v", path, ctx.Err())
		case <-ticker.C:
		}
	}
}

// AssertNoDiagnostics fails when the semantics tree reports accessibility defects.
func AssertNoDiagnostics(t *testing.T, snapshot woxwidget.AutomationSnapshot) {
	t.Helper()
	if len(snapshot.Diagnostics) > 0 {
		t.Fatalf("semantics diagnostics: %v", snapshot.Diagnostics)
	}
}
