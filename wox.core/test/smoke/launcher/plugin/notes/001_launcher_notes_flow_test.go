//go:build wox_ui_smoke

package notes

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test001LauncherNotesFlow verifies the native Notes window preserves a rich note through its core lifecycle.
// Flow: create from Launcher -> enter Markdown -> pin the window -> create an independent note window -> search the saved note -> delete and restore it.
// Evidence: the utility window exposes formatted backing text, window-pin state, persisted search results, trash state, and restored content.
func Test001LauncherNotesFlow(t *testing.T) {
	const title = "Wox Notes Smoke"
	const markdown = "# " + title + "\n- [ ] ship"
	const projected = title + "\n☐ ship"

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
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
		if err := client.Perform(ctx, "notes.editor", woxui.AccessibilityActionSetValue, markdown); err != nil {
			t.Fatalf("enter rich Notes content: %v", err)
		}
		snapshot = waitForEditorValue(t, ctx, client, projected)
		smoke.AssertNoDiagnostics(t, snapshot)
		selectionStart, selectionEnd := clickTaskCheckbox(t, ctx, client, snapshot)
		snapshot = waitForEditorValue(t, ctx, client, title+"\n☑ ship")
		assertEditorSelection(t, snapshot, selectionStart, selectionEnd)
		selectionStart, selectionEnd = clickTaskCheckbox(t, ctx, client, snapshot)
		snapshot = waitForEditorValue(t, ctx, client, projected)
		assertEditorSelection(t, snapshot, selectionStart, selectionEnd)

		if err := client.PressKey(ctx, woxui.Key("p"), primaryModifier()|woxui.KeyModifierShift); err != nil {
			t.Fatalf("pin Notes window: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			pin, found := automationdriver.Find(snapshot, "notes.toolbar.pin")
			return found && (strings.Contains(pin.Label, "Unpin") || strings.Contains(pin.Label, "取消窗口置顶"))
		}); err != nil {
			t.Fatalf("wait for pinned Notes window: %v", err)
		}

		if err := client.Perform(ctx, "notes.toolbar.new", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("create second note: %v", err)
		}
		waitForEditorValue(t, ctx, client, "")
		openSearch(t, ctx, client, title)
		rowID := waitForSearchRow(t, ctx, client, title, false)
		if err := client.Perform(ctx, rowID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("open persisted note from search: %v", err)
		}
		waitForEditorValue(t, ctx, client, projected)

		openMoreMenu(t, ctx, client)
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			_, found := automationdriver.Find(snapshot, "notes.menu.delete")
			return found
		}); err != nil {
			t.Fatalf("wait for Notes delete action: %v", err)
		}
		if err := client.Perform(ctx, "notes.menu.delete", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("delete note: %v", err)
		}
		openSearch(t, ctx, client, title)
		rowID = waitForSearchRow(t, ctx, client, title, true)
		if err := client.Perform(ctx, rowID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("restore deleted note: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			editor, found := automationdriver.Find(snapshot, "notes.editor")
			_, searchOpen := automationdriver.Find(snapshot, "notes.search")
			return found && editor.Value == projected && !searchOpen
		})
		if err != nil {
			t.Fatalf("wait for restored Notes editor: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

func clickTaskCheckbox(t *testing.T, ctx context.Context, client *automationdriver.Client, snapshot woxwidget.AutomationSnapshot) (int, int) {
	t.Helper()
	editor, found := automationdriver.Find(snapshot, "notes.editor")
	if !found {
		t.Fatal("Notes editor is unavailable for task click")
	}
	position := woxui.Point{X: editor.Bounds.X + 20, Y: editor.Bounds.Y + 48}
	for _, kind := range []woxui.PointerEventKind{woxui.PointerDown, woxui.PointerUp} {
		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: kind, Button: woxui.PointerButtonPrimary, Position: position}); err != nil {
			t.Fatalf("click Notes task checkbox: %v", err)
		}
	}
	return editor.SelectionStart, editor.SelectionEnd
}

func assertEditorSelection(t *testing.T, snapshot woxwidget.AutomationSnapshot, start, end int) {
	t.Helper()
	editor, found := automationdriver.Find(snapshot, "notes.editor")
	if !found || editor.SelectionStart != start || editor.SelectionEnd != end {
		t.Fatalf("checkbox click moved Notes selection to %d:%d, want %d:%d", editor.SelectionStart, editor.SelectionEnd, start, end)
	}
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

// openMoreMenu activates the Notes overflow control. The overlay is published on the next frame.
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

// formatNotesSearchNodes reports the live search field and rows after a wait miss.
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

func primaryModifier() woxui.KeyModifiers {
	if runtime.GOOS == "darwin" {
		return woxui.KeyModifierMeta
	}
	return woxui.KeyModifierControl
}
