//go:build wox_ui_smoke

package shell

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test007LauncherQueryShellCommandSettings verifies configured commands remain executable through add, edit, and delete.
// Flow: add a Python command in Shell settings -> run it -> edit and rerun it -> delete it and query again.
// Evidence: the command artifact changes from added to edited, then the deleted alias has no result.
func Test007LauncherQueryShellCommandSettings(t *testing.T) {
	const alias = "woxsmoketable"

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		artifactPath := filepath.Join(os.Getenv(automationdriver.SharedDataDirectoryEnvironment), "shell-command-settings-smoke.txt")
		addedCommand := fmt.Sprintf("open(%q, \"w\").write(%q)", artifactPath, "added")
		editedCommand := fmt.Sprintf("open(%q, \"w\").write(%q)", artifactPath, "edited")

		openShellCommandSettings(t, ctx, client)
		if err := client.Perform(ctx, "plugin-settings-field-2-add", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("add Shell command row: %v", err)
		}
		waitForFormTableRowEditor(t, ctx, client)
		setFormTableRowText(t, ctx, client, 0, alias)
		setFormTableRowText(t, ctx, client, 1, addedCommand)
		smoke.SelectSettingChoiceByLabel(t, ctx, client, "form-table-row-field-2", "Python")
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.Find(snapshot, "form-table-row-field-4")
			return found
		}); err != nil {
			t.Fatalf("wait for Shell command enabled field: %v", err)
		}
		if err := client.Perform(ctx, "form-table-row-field-4", woxui.AccessibilityActionToggle, ""); err != nil {
			t.Fatalf("enable Shell command row: %v", err)
		}
		if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("save added Shell command: %v", err)
		}
		waitForShellCommandRow(t, ctx, client)
		confirmShellCommandRowPersisted(t, ctx, client, "adding", addedCommand)

		runConfiguredShellCommand(t, ctx, client, alias, artifactPath, "added")

		openShellCommandSettings(t, ctx, client)
		waitForShellCommandRow(t, ctx, client)
		if err := client.Perform(ctx, "plugin-settings-field-2-row-0-edit", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("edit Shell command row: %v", err)
		}
		waitForFormTableRowEditor(t, ctx, client)
		setFormTableRowText(t, ctx, client, 1, editedCommand)
		if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("save edited Shell command: %v", err)
		}
		waitForShellCommandRow(t, ctx, client)
		confirmShellCommandRowPersisted(t, ctx, client, "editing", editedCommand)

		runConfiguredShellCommand(t, ctx, client, alias, artifactPath, "edited")

		openShellCommandSettings(t, ctx, client)
		waitForShellCommandRow(t, ctx, client)
		if err := client.Perform(ctx, "plugin-settings-field-2-row-0-delete", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("delete Shell command row: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.Find(snapshot, "form-table-delete-confirm")
			return found
		}); err != nil {
			t.Fatalf("wait for Shell command delete confirmation: %v", err)
		}
		if err := client.Perform(ctx, "form-table-delete-confirm", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("confirm Shell command deletion: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, rowFound := automationdriver.Find(snapshot, "plugin-settings-field-2-row-0-edit")
			_, dialogFound := automationdriver.Find(snapshot, "form-table-delete-dialog")
			return !rowFound && !dialogFound
		}); err != nil {
			t.Fatalf("wait for Shell command row deletion: %v", err)
		}
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close Shell settings after deleting command: %v", err)
		}
		openShellCommandSettings(t, ctx, client)
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, rowFound := automationdriver.Find(snapshot, "plugin-settings-field-2-row-0-edit")
			return !rowFound
		}); err != nil {
			t.Fatalf("confirm Shell command deletion persisted: %v", err)
		}
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close Shell settings after confirming deletion: %v", err)
		}

		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, alias)
		if _, found := shellResult(snapshot, alias); found {
			t.Fatal("deleted Shell command still appears in query results")
		}
	})
}

// openShellCommandSettings opens the Shell command table and waits until its actions are ready.
func openShellCommandSettings(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	openShellPluginSettings(t, ctx, client)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, addFound := automationdriver.Find(snapshot, "plugin-settings-field-2-add")
		return addFound
	}); err != nil {
		t.Fatalf("wait for Shell command settings: %v", err)
	}
}

// waitForFormTableRowEditor waits until the shared row editor can accept values.
func waitForFormTableRowEditor(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, aliasFound := automationdriver.Find(snapshot, "form-table-row-field-0")
		_, commandFound := automationdriver.Find(snapshot, "form-table-row-field-1")
		_, saveFound := automationdriver.Find(snapshot, "form-table-row-save")
		return aliasFound && commandFound && saveFound
	}); err != nil {
		t.Fatalf("wait for Shell command row editor: %v", err)
	}
}

// setFormTableRowText updates one shared table editor text field.
func setFormTableRowText(t *testing.T, ctx context.Context, client *automationdriver.Client, index int, value string) {
	t.Helper()
	id := "form-table-row-field-" + strconv.Itoa(index)
	if err := client.Perform(ctx, id, woxui.AccessibilityActionSetValue, value); err != nil {
		t.Fatalf("set Shell command field %d: %v", index, err)
	}
}

// confirmShellCommandRowPersisted reloads plugin settings before the command is executed.
func confirmShellCommandRowPersisted(t *testing.T, ctx context.Context, client *automationdriver.Client, operation, expectedCommand string) {
	t.Helper()
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close Shell settings after %s command: %v", operation, err)
	}
	openShellCommandSettings(t, ctx, client)
	waitForShellCommandRow(t, ctx, client)
	if err := client.Perform(ctx, "plugin-settings-field-2-row-0-edit", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("inspect Shell command after %s: %v", operation, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		command, commandFound := automationdriver.Find(snapshot, "form-table-row-field-1")
		interpreter, interpreterFound := automationdriver.Find(snapshot, "form-table-row-field-2")
		enabled, enabledFound := automationdriver.Find(snapshot, "form-table-row-field-4")
		silent, silentFound := automationdriver.Find(snapshot, "form-table-row-field-5")
		return commandFound && command.Value == expectedCommand && interpreterFound && interpreter.Value == "Python" && enabledFound && enabled.Checked && silentFound && !silent.Checked
	}); err != nil {
		t.Fatalf("confirm Shell command values after %s: %v", operation, err)
	}
	if err := client.Perform(ctx, "form-table-row-cancel", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("close Shell command inspection after %s: %v", operation, err)
	}
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close Shell settings after confirming %s: %v", operation, err)
	}
}

// waitForShellCommandRow waits for the persisted row actions to return to the settings table.
func waitForShellCommandRow(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, editFound := automationdriver.Find(snapshot, "plugin-settings-field-2-row-0-edit")
		_, deleteFound := automationdriver.Find(snapshot, "plugin-settings-field-2-row-0-delete")
		_, editorFound := automationdriver.Find(snapshot, "form-table-row-save")
		return editFound && deleteFound && !editorFound
	}); err != nil {
		t.Fatalf("wait for Shell command row: %v", err)
	}
}

// runConfiguredShellCommand verifies execution through both result completion metadata and a real file artifact.
func runConfiguredShellCommand(t *testing.T, ctx context.Context, client *automationdriver.Client, alias, artifactPath, expected string) {
	t.Helper()
	smoke.ShowLauncher(t, ctx, client)
	smoke.ReplaceLauncherQuery(t, ctx, client, alias)
	waitForShellResult(t, ctx, client, alias)
	if err := client.PressKey(ctx, woxui.KeyEnter, 0); err != nil {
		t.Fatalf("execute configured Shell command %q: %v", alias, err)
	}
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		resultID, found := shellResult(snapshot, alias)
		if !found {
			return false
		}
		result, found := automationdriver.Find(snapshot, resultID)
		if !found {
			return false
		}
		if _, parseErr := time.Parse("2006-01-02 15:04:05", result.Description); parseErr != nil {
			return false
		}
		return true
	})
	if err != nil {
		t.Fatalf("wait for configured Shell command %q completion: %v", alias, err)
	}
	smoke.WaitForFile(t, ctx, artifactPath, func(data []byte) bool { return string(data) == expected })
	assertShellSnapshot(t, snapshot)
}
