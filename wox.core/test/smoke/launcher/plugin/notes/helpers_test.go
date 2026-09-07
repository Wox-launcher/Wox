//go:build wox_ui_smoke

package notes

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const longChecklistBody = "顺序复制，进入顺序复制模式，标记为1, 2, 3, 4, 5的将按照这个顺序复制，并且继续加长保证在默认笔记窗口宽度下折成多行"

func openNewNoteEditor(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.ShowLauncher(t, ctx, client)
	snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "note new ")
	resultID := selectedResultID(t, snapshot)
	if err := client.Perform(ctx, resultID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("create note from Launcher: %v", err)
	}
	if _, err := client.WaitForWindowState(ctx, "notes", func(state automationdriver.WindowState) bool {
		return state.Exists && state.Visible && state.Lifecycle == "visible"
	}); err != nil {
		t.Fatalf("wait for Notes utility window: %v", err)
	}
	if err := client.FocusInstance(ctx, "notes"); err != nil {
		t.Fatalf("focus Notes utility window: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		editor, found := automationdriver.Find(snapshot, "notes.editor")
		return found && editor.Focused
	}); err != nil {
		t.Fatalf("wait for Notes editor: %v", err)
	}
}

func enterNoteMarkdown(t *testing.T, ctx context.Context, client *automationdriver.Client, markdown, projected string) woxwidget.AutomationSnapshot {
	t.Helper()
	if err := client.Perform(ctx, "notes.editor", woxui.AccessibilityActionSetValue, markdown); err != nil {
		t.Fatalf("enter Notes content: %v", err)
	}
	return waitForEditorValue(t, ctx, client, projected)
}

func waitForEditorVisualLines(t *testing.T, ctx context.Context, client *automationdriver.Client, match func([]woxui.AccessibilityTextLine) bool) []woxui.AccessibilityTextLine {
	t.Helper()
	var lines []woxui.AccessibilityTextLine
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		editor, found := automationdriver.Find(snapshot, "notes.editor")
		if !found {
			return false
		}
		lines = append([]woxui.AccessibilityTextLine(nil), editor.TextLines...)
		return match(editor.TextLines)
	})
	if err != nil {
		editor, found := automationdriver.Find(snapshot, "notes.editor")
		t.Fatalf("wait for Notes visual lines: %v; found=%v lines=%#v value=%q", err, found, editor.TextLines, editor.Value)
	}
	return lines
}

func waitForEditorValue(t *testing.T, ctx context.Context, client *automationdriver.Client, value string) woxwidget.AutomationSnapshot {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		editor, found := automationdriver.Find(snapshot, "notes.editor")
		return found && editor.Value == value
	})
	if err != nil {
		t.Fatalf("wait for Notes editor value %q: %v", value, err)
	}
	return snapshot
}

func selectedResultID(t *testing.T, snapshot woxwidget.AutomationSnapshot) string {
	t.Helper()
	const notesNewID = "launcher.result.notes:new"
	for _, node := range snapshot.Tree.Nodes {
		if node.AutomationID == notesNewID && node.Selected {
			return node.AutomationID
		}
	}
	t.Fatal("Notes new result is not selected")
	return ""
}

func openMoreMenu(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.Perform(ctx, "notes.toolbar.more", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("open Notes menu: %v", err)
	}
}

func openSearch(t *testing.T, ctx context.Context, client *automationdriver.Client, query string) {
	t.Helper()
	if err := client.Perform(ctx, "notes.toolbar.search", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("open Notes search: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		search, found := automationdriver.Find(snapshot, "notes.search")
		return found && search.Focused
	}); err != nil {
		t.Fatalf("wait for Notes search: %v", err)
	}
	if err := client.Perform(ctx, "notes.search", woxui.AccessibilityActionSetValue, query); err != nil {
		t.Fatalf("set Notes search query: %v", err)
	}
}

func waitForSearchRow(t *testing.T, ctx context.Context, client *automationdriver.Client, title string, deleted bool) string {
	t.Helper()
	var rowID string
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "notes.search.") && node.Role == woxui.AccessibilityRoleListItem && strings.Contains(node.Label, title) {
				isDeleted := strings.Contains(node.Label, "Restore") || strings.Contains(node.Label, "恢复")
				if isDeleted == deleted {
					rowID = node.AutomationID
					return true
				}
			}
		}
		return false
	})
	if err != nil {
		t.Fatalf("wait for Notes search result %q (deleted=%v): %v; %s", title, deleted, err, formatNotesSearchNodes(snapshot))
	}
	return rowID
}

func findChecklistVisualLine(lines []woxui.AccessibilityTextLine) (int, bool) {
	for index, line := range lines {
		if strings.Contains(line.Text, "☐") {
			return index, true
		}
	}
	return -1, false
}

func formatNotesSearchNodes(snapshot woxwidget.AutomationSnapshot) string {
	rows := make([]string, 0, 8)
	for _, node := range snapshot.Tree.Nodes {
		if node.AutomationID == "notes.search" || strings.HasPrefix(node.AutomationID, "notes.search.") {
			rows = append(rows, fmt.Sprintf("%s role=%s label=%q value=%q", node.AutomationID, node.Role, node.Label, node.Value))
		}
	}
	if len(rows) == 0 {
		return "search nodes=[]"
	}
	return "search nodes=[" + strings.Join(rows, "; ") + "]"
}
