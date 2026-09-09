//go:build wox_ui_smoke

package notes

import (
	"context"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test001LauncherNotesFlow verifies the native Notes window preserves a rich note through its core lifecycle.
// Flow: create from Launcher -> enter Markdown -> confirm the window is pinned by default -> create an independent note window -> search the saved note -> delete and restore it.
// Evidence: the utility window exposes formatted backing text, window-pin state, persisted search results, trash state, and restored content.
func Test001LauncherNotesFlow(t *testing.T) {
	const title = "Wox Notes Smoke"
	const markdown = "# " + title + "\n- [ ] ship"
	const projected = title + "\n☐ ship"

	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		openNewNoteEditor(t, ctx, client)
		snapshot := enterNoteMarkdown(t, ctx, client, markdown, projected)
		smoke.AssertNoDiagnostics(t, snapshot)
		selectionStart, selectionEnd := clickTaskCheckbox(t, ctx, client, snapshot)
		snapshot = waitForEditorValue(t, ctx, client, title+"\n☑ ship")
		assertEditorSelection(t, snapshot, selectionStart, selectionEnd)
		selectionStart, selectionEnd = clickTaskCheckbox(t, ctx, client, snapshot)
		snapshot = waitForEditorValue(t, ctx, client, projected)
		assertEditorSelection(t, snapshot, selectionStart, selectionEnd)

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
	position := taskCheckboxPoint(snapshot, editor)
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
