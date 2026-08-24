//go:build wox_ui_smoke

package notes

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test001LauncherNotesFlow verifies the native Notes window preserves a rich note through its core lifecycle.
// Flow: create from Launcher -> enter Markdown -> pin -> create an independent note window -> search the saved note -> delete and restore it.
// Evidence: the utility window exposes formatted backing text, pin state, persisted search results, trash state, and restored content.
func Test001LauncherNotesFlow(t *testing.T) {
	const title = "Wox Notes Smoke"
	const markdown = "# " + title + "\n- [ ] ship"
	const projected = title + "\n☐ ship"

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "notes new ")
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
			t.Fatalf("pin note: %v", err)
		}
		if err := client.Perform(ctx, "notes.toolbar.more", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("open Notes menu after pinning: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			pin, found := automationdriver.Find(snapshot, "notes.menu.pin")
			return found && (strings.Contains(pin.Label, "Unpin") || strings.Contains(pin.Label, "取消置顶"))
		}); err != nil {
			t.Fatalf("wait for pinned Notes state: %v", err)
		}
		if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
			t.Fatalf("close Notes menu: %v", err)
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

		if err := client.Perform(ctx, "notes.toolbar.more", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("open Notes menu before deletion: %v", err)
		}
		if err := client.Perform(ctx, "notes.menu.delete", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("delete note: %v", err)
		}
		openSearch(t, ctx, client, title)
		rowID = waitForSearchRow(t, ctx, client, title, true)
		if err := client.Perform(ctx, rowID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("restore deleted note: %v", err)
		}
		snapshot = waitForEditorValue(t, ctx, client, projected)
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
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && node.Selected {
			return node.AutomationID
		}
	}
	t.Fatal("Notes result is not selected")
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
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "notes.search.") && node.Role == woxui.AccessibilityRoleButton && strings.Contains(node.Label, title) {
				isDeleted := strings.Contains(node.Label, "Restore") || strings.Contains(node.Label, "恢复")
				if isDeleted == deleted {
					rowID = node.AutomationID
					return true
				}
			}
		}
		return false
	}); err != nil {
		t.Fatalf("wait for Notes search result %q (deleted=%v): %v", title, deleted, err)
	}
	return rowID
}

func primaryModifier() woxui.KeyModifiers {
	if runtime.GOOS == "darwin" {
		return woxui.KeyModifierMeta
	}
	return woxui.KeyModifierControl
}
