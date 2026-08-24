//go:build wox_ui_smoke

package shell

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const shellPluginID = "8a4b5c6d-7e8f-9a0b-1c2d-3e4f5a6b7c8d"

func shellResult(snapshot woxwidget.AutomationSnapshot, command string) (string, bool) {
	return smoke.FindLauncherResult(snapshot, command)
}

func waitForShellResult(t *testing.T, ctx context.Context, client *automationdriver.Client, title string) string {
	t.Helper()
	var resultID string
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		resultID, _ = shellResult(snapshot, title)
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		return resultID != "" && resultsFound && results.Value == "complete"
	})
	if err != nil {
		t.Fatalf("wait for Shell result %q: %v", title, err)
	}
	assertShellSnapshot(t, snapshot)
	return resultID
}

// selectShellResult moves launcher selection through the native keyboard path until the target is selected.
func selectShellResult(t *testing.T, ctx context.Context, client *automationdriver.Client, resultID string) woxwidget.AutomationSnapshot {
	t.Helper()
	return smoke.SelectLauncherResult(t, ctx, client, resultID)
}

func waitForTerminalStatus(t *testing.T, ctx context.Context, client *automationdriver.Client, status string) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		node, found := automationdriver.Find(snapshot, "launcher.preview.terminal.status")
		return found && node.Value == status
	})
	if err != nil {
		t.Fatalf("wait for terminal status %q: %v", status, err)
	}
	assertShellSnapshot(t, snapshot)
	return snapshot
}

func activateShellAction(t *testing.T, ctx context.Context, client *automationdriver.Client, actionPrefix string) {
	t.Helper()
	smoke.ActivateSelectedResultAction(t, ctx, client, actionPrefix)
}

func assertShellSnapshot(t *testing.T, snapshot woxwidget.AutomationSnapshot) {
	t.Helper()
	smoke.AssertNoDiagnostics(t, snapshot)
}

func openShellPluginSettings(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.OpenInstalledPluginSettings(t, ctx, client, shellPluginID)
}

const (
	shellWorkingDirectoryModeFieldID    = "plugin-settings-field-2"
	shellCustomWorkingDirectoryFieldID  = "plugin-settings-field-3"
	shellWorkingDirectoryPreviewTagID   = "preview-tag-1"
	shellChangeWorkingDirectoryAction   = "action-result-change_working_directory-"
	shellCustomWorkingDirectoryOption   = 2
	shellHomeWorkingDirectoryOption     = 0
	shellLastUsedWorkingDirectoryOption = 1
	shellCommandsTableAddID             = "plugin-settings-field-3-add"
	shellCustomCommandsTableAddID       = "plugin-settings-field-4-add"
)

// shellWriteCwdMarkerCommand writes a marker file in the process working directory.
func shellWriteCwdMarkerCommand(marker string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("Set-Content -LiteralPath %s -Value cwd -Encoding ascii", marker)
	}
	return fmt.Sprintf("printf cwd > %s", marker)
}

func shellPrintWorkingDirectoryCommand() string {
	if runtime.GOOS == "windows" {
		return "Write-Output $PWD.Path"
	}
	return "pwd"
}

func sameShellWorkingDirectory(got, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" {
		return false
	}
	gotAbs, gotErr := filepath.Abs(got)
	wantAbs, wantErr := filepath.Abs(want)
	if gotErr != nil || wantErr != nil {
		return filepath.Clean(got) == filepath.Clean(want)
	}
	gotAbs = filepath.Clean(gotAbs)
	wantAbs = filepath.Clean(wantAbs)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(gotAbs, wantAbs)
	}
	return gotAbs == wantAbs
}

func waitForShellWorkingDirectoryMode(t *testing.T, ctx context.Context, client *automationdriver.Client) string {
	t.Helper()
	var current string
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, shellWorkingDirectoryModeFieldID)
		if found {
			current = field.Value
		}
		return found && current != ""
	}); err != nil {
		t.Fatalf("wait for Shell working directory mode: %v", err)
	}
	return current
}

func selectPluginSettingChoiceByIndex(t *testing.T, ctx context.Context, client *automationdriver.Client, fieldID string, optionIndex int) string {
	t.Helper()
	if err := client.Perform(ctx, fieldID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("open plugin setting %q: %v", fieldID, err)
	}
	optionID := fmt.Sprintf("setting-choice-%d", optionIndex)
	var label string
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		choice, found := automationdriver.Find(snapshot, optionID)
		_, menuFound := automationdriver.Find(snapshot, "setting-choice-menu")
		if found {
			label = choice.Label
		}
		return menuFound && found && label != ""
	}); err != nil {
		t.Fatalf("wait for plugin setting %q choice %d: %v", fieldID, optionIndex, err)
	}
	if err := client.Perform(ctx, optionID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("select plugin setting %q choice %d: %v", fieldID, optionIndex, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		control, found := automationdriver.Find(snapshot, fieldID)
		_, menuFound := automationdriver.Find(snapshot, "setting-choice-menu")
		return found && control.Value == label && !menuFound
	}); err != nil {
		t.Fatalf("wait for plugin setting %q choice %d to commit: %v", fieldID, optionIndex, err)
	}
	return label
}

func waitForShellCustomWorkingDirectoryField(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, pathFound := automationdriver.Find(snapshot, shellCustomWorkingDirectoryFieldID)
		_, commandsAtPath := automationdriver.Find(snapshot, shellCommandsTableAddID)
		_, commandsMoved := automationdriver.Find(snapshot, shellCustomCommandsTableAddID)
		return pathFound && !commandsAtPath && commandsMoved
	}); err != nil {
		t.Fatalf("wait for custom Shell working directory field: %v", err)
	}
}

func waitForShellDefaultWorkingDirectoryWithoutPath(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, commandsAtPath := automationdriver.Find(snapshot, shellCommandsTableAddID)
		_, customPath := automationdriver.Find(snapshot, shellCustomCommandsTableAddID)
		return commandsAtPath && !customPath
	}); err != nil {
		t.Fatalf("wait for Shell working directory mode without a custom path: %v", err)
	}
}

func setShellCustomWorkingDirectoryPath(t *testing.T, ctx context.Context, client *automationdriver.Client, workingDirectory string) {
	t.Helper()
	if err := client.Perform(ctx, shellCustomWorkingDirectoryFieldID, woxui.AccessibilityActionSetValue, workingDirectory); err != nil {
		t.Fatalf("set custom Shell working directory: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, shellCustomWorkingDirectoryFieldID)
		return found && sameShellWorkingDirectory(field.Value, workingDirectory)
	}); err != nil {
		t.Fatalf("wait for custom Shell working directory %q: %v", workingDirectory, err)
	}
}

func selectShellWorkingDirectoryMode(t *testing.T, ctx context.Context, client *automationdriver.Client, optionIndex int) string {
	t.Helper()
	openShellPluginSettings(t, ctx, client)
	waitForShellWorkingDirectoryMode(t, ctx, client)
	label := selectPluginSettingChoiceByIndex(t, ctx, client, shellWorkingDirectoryModeFieldID, optionIndex)
	if optionIndex == shellCustomWorkingDirectoryOption {
		waitForShellCustomWorkingDirectoryField(t, ctx, client)
	} else {
		waitForShellDefaultWorkingDirectoryWithoutPath(t, ctx, client)
	}
	return label
}

func confirmShellWorkingDirectoryModePersisted(t *testing.T, ctx context.Context, client *automationdriver.Client, expectedLabel string, custom bool) {
	t.Helper()
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close Shell settings after selecting working directory mode: %v", err)
	}
	openShellPluginSettings(t, ctx, client)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, shellWorkingDirectoryModeFieldID)
		if !found || field.Value != expectedLabel {
			return false
		}
		_, commandsAtPath := automationdriver.Find(snapshot, shellCommandsTableAddID)
		_, customPath := automationdriver.Find(snapshot, shellCustomCommandsTableAddID)
		if custom {
			return !commandsAtPath && customPath
		}
		return commandsAtPath && !customPath
	}); err != nil {
		t.Fatalf("confirm persisted Shell working directory mode %q: %v", expectedLabel, err)
	}
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close Shell settings after confirming working directory mode: %v", err)
	}
}

func configureShellCustomWorkingDirectory(t *testing.T, ctx context.Context, client *automationdriver.Client, workingDirectory string) {
	t.Helper()
	selectShellWorkingDirectoryMode(t, ctx, client, shellCustomWorkingDirectoryOption)
	setShellCustomWorkingDirectoryPath(t, ctx, client, workingDirectory)
}

func confirmShellCustomWorkingDirectoryPersisted(t *testing.T, ctx context.Context, client *automationdriver.Client, workingDirectory string) {
	t.Helper()
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close Shell settings after setting custom directory: %v", err)
	}
	openShellPluginSettings(t, ctx, client)
	waitForShellWorkingDirectoryMode(t, ctx, client)
	waitForShellCustomWorkingDirectoryField(t, ctx, client)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, shellCustomWorkingDirectoryFieldID)
		return found && sameShellWorkingDirectory(field.Value, workingDirectory)
	}); err != nil {
		t.Fatalf("confirm persisted custom Shell working directory %q: %v", workingDirectory, err)
	}
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close Shell settings after confirming custom directory: %v", err)
	}
}

func restoreShellHomeWorkingDirectory(t *testing.T, client *automationdriver.Client) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), automationdriver.ActionTimeout)
	defer cancel()
	if err := client.Hide(ctx); err != nil {
		t.Errorf("hide windows before restoring Shell working directory: %v", err)
	}
	selectShellWorkingDirectoryMode(t, ctx, client, shellHomeWorkingDirectoryOption)
	if err := client.Hide(ctx); err != nil {
		t.Errorf("close Shell settings after restoring home directory: %v", err)
	}
}

func waitForShellPreviewWorkingDirectory(t *testing.T, ctx context.Context, client *automationdriver.Client, workingDirectory string) {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		tag, found := automationdriver.Find(snapshot, shellWorkingDirectoryPreviewTagID)
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		return found && resultsFound && results.Value == "complete" && sameShellWorkingDirectory(tag.Label, workingDirectory)
	})
	if err != nil {
		t.Fatalf("wait for Shell preview working directory %q: %v", workingDirectory, err)
	}
	assertShellSnapshot(t, snapshot)
}

func waitForShellTerminalOutputDirectory(t *testing.T, ctx context.Context, client *automationdriver.Client, workingDirectory string) {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		status, statusFound := automationdriver.Find(snapshot, "launcher.preview.terminal.status")
		output, outputFound := automationdriver.Find(snapshot, "launcher.preview.terminal.output")
		if !statusFound || status.Value != "completed" || !outputFound {
			return false
		}
		for _, line := range strings.Split(output.Value, "\n") {
			if sameShellWorkingDirectory(line, workingDirectory) {
				return true
			}
		}
		return false
	})
	if err != nil {
		t.Fatalf("wait for Shell terminal working directory %q: %v", workingDirectory, err)
	}
	assertShellSnapshot(t, snapshot)
}

func queryShellWorkingDirectory(t *testing.T, ctx context.Context, client *automationdriver.Client, query, workingDirectory string) {
	t.Helper()
	smoke.ShowLauncher(t, ctx, client)
	smoke.ReplaceLauncherQuery(t, ctx, client, query)
	waitForShellPreviewWorkingDirectory(t, ctx, client, workingDirectory)
}

func executeShellCwdMarker(t *testing.T, ctx context.Context, client *automationdriver.Client, command, workingDirectory, marker string) {
	t.Helper()
	resultID := waitForShellResult(t, ctx, client, command)
	if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("execute Shell cwd command %q: %v", command, err)
	}
	waitForTerminalStatus(t, ctx, client, "completed")
	smoke.WaitForFile(t, ctx, filepath.Join(workingDirectory, marker), func(data []byte) bool {
		return strings.TrimSpace(string(data)) == "cwd"
	})
}
