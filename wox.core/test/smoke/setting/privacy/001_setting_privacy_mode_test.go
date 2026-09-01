//go:build wox_ui_smoke

package privacy

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const historyQuery = "1+1"

// lifecycleState carries the original and changed values across cleanup-owned process restarts.
type lifecycleState struct {
	OriginalLanguage string `json:"original_language"`
	PrivateLanguage  string `json:"private_language"`
	NormalLanguage   string `json:"normal_language"`
	OriginalMaximum  string `json:"original_maximum"`
	ChangedMaximum   string `json:"changed_maximum"`
}

// Test001SettingPrivacyMode verifies private cleanup and normal persistence across real Wox restarts.
// Flow: enable private mode -> change language, result limit, and query history -> restart -> repeat with private mode disabled -> restart.
// Evidence: private restart keeps language but clears the result limit and history, while normal restart keeps all three changes.
func Test001SettingPrivacyMode(t *testing.T) {
	phase, err := strconv.Atoi(os.Getenv(automationdriver.SharedLifecyclePhaseEnvironment))
	if err != nil || phase < 1 || phase > 4 {
		t.Fatalf("invalid privacy lifecycle phase %q", os.Getenv(automationdriver.SharedLifecyclePhaseEnvironment))
	}
	ctx, cancel := context.WithTimeout(context.Background(), smoke.CaseTimeout)
	defer cancel()
	client := smoke.SharedClient(t, ctx)
	if err := client.Reset(ctx); err != nil {
		t.Fatalf("reset Wox before privacy lifecycle phase %d: %v", phase, err)
	}

	switch phase {
	case 1:
		startPrivateSession(t, ctx, client)
	case 2:
		verifyPrivateSessionAndStartNormalSession(t, ctx, client)
	case 3:
		verifyNormalSessionAndStartCleanup(t, ctx, client)
	case 4:
		verifyCleanupAndRestore(t, ctx, client)
	}
}

// startPrivateSession records the baseline, enables private mode, and exits after representative changes.
func startPrivateSession(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	state := lifecycleState{
		OriginalLanguage: smoke.OpenSettingsAndReadChoice(t, ctx, client, "/general", "LangCode"),
		OriginalMaximum:  smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "MaxResultCount"),
	}
	state.PrivateLanguage = differentLanguage(state.OriginalLanguage)
	state.NormalLanguage = differentLanguage(state.PrivateLanguage)
	state.ChangedMaximum = differentMaximum(state.OriginalMaximum)
	writeLifecycleState(t, state)

	openPrivacySettings(t, ctx, client)
	setPrivacyMode(t, ctx, client, true)
	smoke.OpenSettingsAndReadChoice(t, ctx, client, "/general", "LangCode")
	smoke.SelectSettingChoiceByLabel(t, ctx, client, "setting-choice-LangCode", state.PrivateLanguage)
	smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "MaxResultCount")
	smoke.SelectSettingChoiceByLabel(t, ctx, client, "setting-choice-MaxResultCount", state.ChangedMaximum)
	recordSettingsQuery(t, ctx, client)
	exitWox(t, ctx, client, state.PrivateLanguage)
}

// verifyPrivateSessionAndStartNormalSession checks cleanup, disables private mode, and creates normally persisted changes.
func verifyPrivateSessionAndStartNormalSession(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	state := readLifecycleState(t)
	if !openPrivacySettings(t, ctx, client) {
		t.Fatal("private mode was disabled after the private restart")
	}
	assertChoice(t, smoke.OpenSettingsAndReadChoice(t, ctx, client, "/general", "LangCode"), state.PrivateLanguage, "preserved language")
	assertChoiceChanged(t, smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "MaxResultCount"), state.ChangedMaximum, "private result limit")
	assertQueryHistory(t, ctx, client, false)

	openPrivacySettings(t, ctx, client)
	setPrivacyMode(t, ctx, client, false)
	smoke.OpenSettingsAndReadChoice(t, ctx, client, "/general", "LangCode")
	smoke.SelectSettingChoiceByLabel(t, ctx, client, "setting-choice-LangCode", state.NormalLanguage)
	smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "MaxResultCount")
	smoke.SelectSettingChoiceByLabel(t, ctx, client, "setting-choice-MaxResultCount", state.ChangedMaximum)
	recordSettingsQuery(t, ctx, client)
	exitWox(t, ctx, client, state.NormalLanguage)
}

// verifyNormalSessionAndStartCleanup checks normal persistence, then starts one final private cleanup for suite isolation.
func verifyNormalSessionAndStartCleanup(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	state := readLifecycleState(t)
	if openPrivacySettings(t, ctx, client) {
		t.Fatal("private mode was enabled after the normal restart")
	}
	assertChoice(t, smoke.OpenSettingsAndReadChoice(t, ctx, client, "/general", "LangCode"), state.NormalLanguage, "normal language")
	assertChoice(t, smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "MaxResultCount"), state.ChangedMaximum, "normal result limit")
	assertQueryHistory(t, ctx, client, true)

	openPrivacySettings(t, ctx, client)
	setPrivacyMode(t, ctx, client, true)
	exitWox(t, ctx, client, state.NormalLanguage)
}

// verifyCleanupAndRestore leaves the shared suite with private mode off and its original settings restored.
func verifyCleanupAndRestore(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	state := readLifecycleState(t)
	if !openPrivacySettings(t, ctx, client) {
		t.Fatal("private mode was disabled before final cleanup verification")
	}
	assertChoice(t, smoke.OpenSettingsAndReadChoice(t, ctx, client, "/general", "LangCode"), state.NormalLanguage, "cleanup language")
	assertChoiceChanged(t, smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "MaxResultCount"), state.ChangedMaximum, "cleanup result limit")
	assertQueryHistory(t, ctx, client, false)

	openPrivacySettings(t, ctx, client)
	setPrivacyMode(t, ctx, client, false)
	smoke.OpenSettingsAndReadChoice(t, ctx, client, "/general", "LangCode")
	smoke.SelectSettingChoiceByLabel(t, ctx, client, "setting-choice-LangCode", state.OriginalLanguage)
	smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "MaxResultCount")
	smoke.SelectSettingChoiceByLabel(t, ctx, client, "setting-choice-MaxResultCount", state.OriginalMaximum)
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read restored Privacy settings: %v", err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close restored Privacy settings: %v", err)
	}
}

// openPrivacySettings opens the owning page and returns the current private-mode state.
func openPrivacySettings(t *testing.T, ctx context.Context, client *automationdriver.Client) bool {
	t.Helper()
	if err := client.OpenSettings(ctx, "/privacy"); err != nil {
		t.Fatalf("open Privacy settings: %v", err)
	}
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, pageFound := automationdriver.Find(snapshot, "settings.page.privacy")
		_, switchFound := automationdriver.Find(snapshot, "privacy-mode-switch")
		return pageFound && switchFound
	})
	if err != nil {
		t.Fatalf("wait for Privacy settings: %v", err)
	}
	privacySwitch, _ := automationdriver.Find(snapshot, "privacy-mode-switch")
	return privacySwitch.Checked
}

// setPrivacyMode drives the real switch and waits for the committed UI state.
func setPrivacyMode(t *testing.T, ctx context.Context, client *automationdriver.Client, enabled bool) {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read privacy mode switch: %v", err)
	}
	privacySwitch, found := automationdriver.Find(snapshot, "privacy-mode-switch")
	if !found {
		t.Fatal("privacy mode switch was not found")
	}
	if privacySwitch.Checked != enabled {
		if err := client.Perform(ctx, "privacy-mode-switch", woxui.AccessibilityActionToggle, ""); err != nil {
			t.Fatalf("toggle privacy mode to %t: %v", enabled, err)
		}
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		privacySwitch, found := automationdriver.Find(snapshot, "privacy-mode-switch")
		return found && privacySwitch.Checked == enabled
	}); err != nil {
		t.Fatalf("wait for privacy mode to become %t: %v", enabled, err)
	}
}

// recordSettingsQuery executes a harmless real result so the query enters persisted history.
func recordSettingsQuery(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.PreserveClipboard(t)
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close settings before recording query history: %v", err)
	}
	smoke.ShowLauncher(t, ctx, client)
	snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, historyQuery)
	resultID := resultIDByLabel(t, snapshot, "2", historyQuery)
	if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("activate calculator result to record query history: %v", err)
	}
	// Activating a result hides the launcher asynchronously, so the next show has to
	// wait for a settled hidden window instead of racing it. Losing that race left
	// the launcher hidden while its last frame still advertised the activated query
	// and result, which no later wait could clear.
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide launcher after recording query history: %v", err)
	}
	if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
		return state.Exists && !state.Visible
	}); err != nil {
		t.Fatalf("wait for launcher to hide after recording query history: %v", err)
	}
	waitForRecordedHistory(t, ctx, client)
}

// waitForRecordedHistory waits for the action's asynchronous persistence through the Query History plugin.
func waitForRecordedHistory(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.ShowLauncher(t, ctx, client)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	query := "h " + historyQuery
	for {
		snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, query)
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "launcher.result.") && node.Label == historyQuery {
				return
			}
		}
		if strings.HasSuffix(query, " ") {
			query = strings.TrimSuffix(query, " ")
		} else {
			query += " "
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for query history persistence: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

// assertQueryHistory checks the user-visible Query History plugin after a restart.
func assertQueryHistory(t *testing.T, ctx context.Context, client *automationdriver.Client, expected bool) {
	t.Helper()
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close settings before checking query history: %v", err)
	}
	smoke.ShowLauncher(t, ctx, client)
	if expected {
		snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "h "+historyQuery)
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "launcher.result.") && node.Label == historyQuery {
				return
			}
		}
		t.Fatalf("query history %q was not visible after normal restart", historyQuery)
	}
	if err := client.PressKey(ctx, woxui.KeyArrowUp, 0); err != nil {
		t.Fatalf("recall query history after restart: %v", err)
	}
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read recalled query history: %v", err)
	}
	query, found := automationdriver.Find(snapshot, "launcher.query.input")
	if !found {
		t.Fatal("launcher query input was not found while checking history")
	}
	if query.Value != "" {
		t.Fatalf("recalled query after private restart = %q, want empty", query.Value)
	}
}

// exitWox activates the real Exit command and waits until the automation endpoint disappears.
func exitWox(t *testing.T, ctx context.Context, client *automationdriver.Client, language string) {
	t.Helper()
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close active window before exiting Wox: %v", err)
	}
	smoke.ShowLauncher(t, ctx, client)
	if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, "exit wox"); err != nil {
		return
	}
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		query, queryFound := automationdriver.Find(snapshot, "launcher.query.input")
		if !queryFound || query.Value != "exit wox" {
			return false
		}
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "launcher.result.") && node.Label == exitLabel(language) {
				return true
			}
		}
		return false
	})
	if err != nil {
		if _, snapshotErr := client.Snapshot(ctx); snapshotErr != nil {
			return
		}
		t.Fatalf("wait for Exit result: %v", err)
	}
	resultID := resultIDByLabel(t, snapshot, exitLabel(language), "exit wox")
	_ = client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, "")
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := client.Snapshot(ctx); err != nil {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for Wox to exit: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

// resultIDByLabel returns the stable generation-local ID for one exact result label.
func resultIDByLabel(t *testing.T, snapshot woxwidget.AutomationSnapshot, label, query string) string {
	t.Helper()
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && node.Label == label {
			return node.AutomationID
		}
	}
	t.Fatalf("query %q has no result labeled %q", query, label)
	return ""
}

func exitLabel(language string) string {
	if language == "简体中文" {
		return "退出Wox"
	}
	return "Exit"
}

func differentLanguage(current string) string {
	if current == "English" {
		return "简体中文"
	}
	return "English"
}

func differentMaximum(current string) string {
	if current == "5" {
		return "15"
	}
	return "5"
}

// writeLifecycleState persists phase expectations outside Wox's private cleanup root.
func writeLifecycleState(t *testing.T, state lifecycleState) {
	t.Helper()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("encode privacy lifecycle state: %v", err)
	}
	if err := os.WriteFile(os.Getenv(automationdriver.SharedLifecycleStateEnvironment), data, 0600); err != nil {
		t.Fatalf("write privacy lifecycle state: %v", err)
	}
}

// readLifecycleState loads the expectations recorded before the first restart.
func readLifecycleState(t *testing.T) lifecycleState {
	t.Helper()
	data, err := os.ReadFile(os.Getenv(automationdriver.SharedLifecycleStateEnvironment))
	if err != nil {
		t.Fatalf("read privacy lifecycle state: %v", err)
	}
	var state lifecycleState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode privacy lifecycle state: %v", err)
	}
	return state
}

func assertChoice(t *testing.T, actual, expected, name string) {
	t.Helper()
	if actual != expected {
		t.Fatalf("%s after restart = %q, want %q", name, actual, expected)
	}
}

func assertChoiceChanged(t *testing.T, actual, discarded, name string) {
	t.Helper()
	if actual == discarded {
		t.Fatalf("%s survived private cleanup as %q", name, actual)
	}
}
