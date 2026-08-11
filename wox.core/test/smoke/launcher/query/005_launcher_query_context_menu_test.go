//go:build wox_ui_smoke

package query

import (
	"context"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/clipboard"
)

const (
	queryContextMenuSample   = "smoke-query-context-menu"
	queryContextMenuCutID    = "launcher.query.menu.cut"
	queryContextMenuCopyID   = "launcher.query.menu.copy"
	queryContextMenuPasteID  = "launcher.query.menu.paste"
	queryContextMenuSelectID = "launcher.query.menu.selectAll"
)

// Test005LauncherQueryContextMenu verifies the query-box host overlay context menu Cut/Copy/Paste/Select All flow.
// Flow: show launcher -> set query text -> secondary-click the query -> Select All -> reopen menu -> Copy -> Cut -> Paste.
// Evidence: menu items expose stable IDs and enablement, clipboard receives the selected text, Cut clears the query, and Paste restores it.
func Test005LauncherQueryContextMenu(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.PreserveClipboard(t)
		smoke.ShowLauncher(t, ctx, client)

		if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, queryContextMenuSample); err != nil {
			t.Fatalf("set query text: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			return found && input.Value == queryContextMenuSample
		}); err != nil {
			t.Fatalf("wait for query text: %v", err)
		}

		openQueryContextMenu(t, ctx, client)
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			cut, cutFound := automationdriver.Find(snapshot, queryContextMenuCutID)
			copyItem, copyFound := automationdriver.Find(snapshot, queryContextMenuCopyID)
			paste, pasteFound := automationdriver.Find(snapshot, queryContextMenuPasteID)
			selectAll, selectFound := automationdriver.Find(snapshot, queryContextMenuSelectID)
			return cutFound && copyFound && pasteFound && selectFound &&
				!cut.Enabled && !copyItem.Enabled && paste.Enabled && selectAll.Enabled
		}); err != nil {
			t.Fatalf("wait for initial query context menu enablement: %v", err)
		}

		if err := client.Perform(ctx, queryContextMenuSelectID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("activate Select All menu item: %v", err)
		}
		runeCount := len([]rune(queryContextMenuSample))
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			_, menuOpen := automationdriver.Find(snapshot, queryContextMenuSelectID)
			return found && !menuOpen && input.HasTextSelection &&
				input.SelectionStart == 0 && input.SelectionEnd == runeCount
		}); err != nil {
			t.Fatalf("wait for Select All from context menu: %v", err)
		}

		openQueryContextMenu(t, ctx, client)
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			cut, cutFound := automationdriver.Find(snapshot, queryContextMenuCutID)
			copyItem, copyFound := automationdriver.Find(snapshot, queryContextMenuCopyID)
			return cutFound && copyFound && cut.Enabled && copyItem.Enabled
		}); err != nil {
			t.Fatalf("wait for Cut/Copy enablement after selection: %v", err)
		}
		if err := client.Perform(ctx, queryContextMenuCopyID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("activate Copy menu item: %v", err)
		}
		if text, err := clipboard.ReadText(); err != nil || text != queryContextMenuSample {
			t.Fatalf("clipboard after menu copy = %q err %v, want %q", text, err, queryContextMenuSample)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			_, menuOpen := automationdriver.Find(snapshot, queryContextMenuCopyID)
			return found && !menuOpen && input.Value == queryContextMenuSample
		}); err != nil {
			t.Fatalf("wait for query to remain after menu copy: %v", err)
		}

		openQueryContextMenu(t, ctx, client)
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			cut, found := automationdriver.Find(snapshot, queryContextMenuCutID)
			return found && cut.Enabled
		}); err != nil {
			t.Fatalf("wait for Cut menu item after copy: %v", err)
		}
		if err := client.Perform(ctx, queryContextMenuCutID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("activate Cut menu item: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			_, menuOpen := automationdriver.Find(snapshot, queryContextMenuCutID)
			return found && !menuOpen && input.Value == ""
		}); err != nil {
			t.Fatalf("wait for empty query after menu cut: %v", err)
		}
		if text, err := clipboard.ReadText(); err != nil || text != queryContextMenuSample {
			t.Fatalf("clipboard after menu cut = %q err %v, want %q", text, err, queryContextMenuSample)
		}

		openQueryContextMenu(t, ctx, client)
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			paste, found := automationdriver.Find(snapshot, queryContextMenuPasteID)
			return found && paste.Enabled
		}); err != nil {
			t.Fatalf("wait for Paste menu item: %v", err)
		}
		if err := client.Perform(ctx, queryContextMenuPasteID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("activate Paste menu item: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			input, found := automationdriver.Find(snapshot, "launcher.query.input")
			_, menuOpen := automationdriver.Find(snapshot, queryContextMenuPasteID)
			return found && !menuOpen && input.Value == queryContextMenuSample
		})
		if err != nil {
			t.Fatalf("wait for restored query after menu paste: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// openQueryContextMenu secondary-clicks the query input using its live semantic bounds.
func openQueryContextMenu(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot before query context menu: %v", err)
	}
	input, found := automationdriver.Find(snapshot, "launcher.query.input")
	if !found {
		t.Fatal("launcher query input missing before context menu")
	}
	position := woxui.Point{X: input.Bounds.X + min(float32(24), input.Bounds.Width/2), Y: input.Bounds.Y + input.Bounds.Height/2}
	if err := client.Pointer(ctx, woxui.PointerEvent{
		Kind: woxui.PointerDown, Button: woxui.PointerButtonSecondary, Position: position,
	}); err != nil {
		t.Fatalf("secondary press on query input: %v", err)
	}
}
