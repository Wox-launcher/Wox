//go:build wox_ui_smoke

package notes

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test005NotesSidebarTree exercises nested organization through the native window and pointer routing.
func Test005NotesSidebarTree(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		openNewNoteEditor(t, ctx, client)
		enterNoteMarkdown(t, ctx, client, "Sidebar original", "Sidebar original")
		restore := widenNotesWindow(t, ctx, client)
		defer restore()
		activate := func(id string) {
			t.Helper()
			if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool { _, ok := automationdriver.Find(s, id); return ok }); err != nil {
				t.Fatalf("wait for %s: %v", id, err)
			}
			if err := client.Perform(ctx, id, woxui.AccessibilityActionActivate, ""); err != nil {
				t.Fatalf("activate %s: %v", id, err)
			}
		}
		row := func(title string) woxui.AccessibilityNode {
			t.Helper()
			var found woxui.AccessibilityNode
			if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
				for _, n := range s.Tree.Nodes {
					if strings.HasPrefix(n.AutomationID, "notes.sidebar.row.") && n.Label == title {
						found = n
						return true
					}
				}
				return false
			}); err != nil {
				t.Fatalf("wait for tree row %q: %v", title, err)
			}
			return found
		}
		createGroup := func(name string) {
			t.Helper()
			activate("notes.sidebar.group")
			if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
				_, ok := automationdriver.Find(s, "notes.group.name")
				return ok
			}); err != nil {
				t.Fatal(err)
			}
			if name == "Sidebar A" && os.Getenv("WOX_SMOKE_ARTIFACT_DIR") != "" {
				if err := client.RequestFrame(ctx); err != nil {
					t.Fatal(err)
				}
				if err := client.Capture(ctx, filepath.Join(os.Getenv("WOX_SMOKE_ARTIFACT_DIR"), "notes-group-dialog.png")); err != nil {
					t.Fatal(err)
				}
			}
			if name == "Sidebar A" {
				activate("notes.organize.save")
			}
			if err := client.Perform(ctx, "notes.group.name", woxui.AccessibilityActionSetValue, name); err != nil {
				t.Fatal(err)
			}
			if name == "Sidebar A" {
				if err := client.Perform(ctx, "notes.group.name", woxui.AccessibilityActionFocus, ""); err != nil {
					t.Fatal(err)
				}
				if err := client.PressKey(ctx, woxui.KeyEnter, 0); err != nil {
					t.Fatal(err)
				}
			} else {
				activate("notes.organize.save")
			}
		}
		activate("notes.toolbar.sidebar")
		createGroup("Sidebar A")
		activate(row("Sidebar A").AutomationID)
		createGroup("Sidebar B")
		activate(row("Sidebar B").AutomationID)
		activate("notes.sidebar.new")
		enterNoteMarkdown(t, ctx, client, "Sidebar nested note", "Sidebar nested note")
		note := row("Sidebar nested note")
		b := row("Sidebar B")
		a := row("Sidebar A")
		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonSecondary, Position: woxui.Point{X: note.Bounds.X + note.Bounds.Width/2, Y: note.Bounds.Y + note.Bounds.Height/2}}); err != nil {
			t.Fatal(err)
		}
		menuSnapshot, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			_, ok := automationdriver.Find(s, "notes.organize.menu")
			return ok
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"notes.organize.open", "notes.organize.move"} {
			if _, found := automationdriver.Find(menuSnapshot, id); found {
				t.Fatalf("redundant note action %s is present", id)
			}
		}
		if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
			t.Fatal(err)
		}
		visible := func(id string, expected bool) {
			t.Helper()
			if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
				_, ok := automationdriver.Find(s, id)
				return ok == expected
			}); err != nil {
				t.Fatalf("visibility %s = %v: %v", id, expected, err)
			}
		}
		activate("notes.sidebar.toggle-all")
		visible(b.AutomationID, false)
		activate("notes.sidebar.toggle-all")
		visible(b.AutomationID, true)
		visible(note.AutomationID, true)
		activate(b.AutomationID)
		visible(note.AutomationID, false)
		activate(b.AutomationID)
		visible(note.AutomationID, true)
		capturedDrag := false
		drag := func(source, target woxui.AccessibilityNode) {
			t.Helper()
			start := woxui.Point{X: source.Bounds.X + source.Bounds.Width/2, Y: source.Bounds.Y + source.Bounds.Height/2}
			end := woxui.Point{X: target.Bounds.X + target.Bounds.Width/2, Y: target.Bounds.Y + target.Bounds.Height/2}
			for _, event := range []woxui.PointerEvent{{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: start}, {Kind: woxui.PointerMove, Position: end}, {Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: end}} {
				if err := client.Pointer(ctx, event); err != nil {
					t.Fatal(err)
				}
				if event.Kind == woxui.PointerMove {
					if err := client.RequestFrame(ctx); err != nil {
						t.Fatal(err)
					}
					if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
						preview, ok := automationdriver.Find(s, "notes.sidebar.drag.preview")
						return ok && preview.Bounds.Width > 0 && preview.Bounds.Height > 0 && end.X >= preview.Bounds.X && end.X <= preview.Bounds.X+preview.Bounds.Width && end.Y >= preview.Bounds.Y && end.Y <= preview.Bounds.Y+preview.Bounds.Height
					}); err != nil {
						t.Fatalf("drag preview missing: %v", err)
					}
					if captureDir := os.Getenv("WOX_SMOKE_ARTIFACT_DIR"); captureDir != "" && !capturedDrag {
						if err := client.Capture(ctx, filepath.Join(captureDir, "notes-sidebar-drag.png")); err != nil {
							t.Fatal(err)
						}
						capturedDrag = true
					}
				}
			}
		}
		start := woxui.Point{X: note.Bounds.X + note.Bounds.Width/2, Y: note.Bounds.Y + note.Bounds.Height/2}
		end := woxui.Point{X: a.Bounds.X + a.Bounds.Width/2, Y: a.Bounds.Y + a.Bounds.Height/2}
		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: start}); err != nil {
			t.Fatal(err)
		}
		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerMove, Position: end}); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			_, ok := automationdriver.Find(s, "notes.sidebar.drag.preview")
			return ok
		}); err != nil {
			t.Fatalf("drag preview missing before cancellation: %v", err)
		}
		if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			_, ok := automationdriver.Find(s, "notes.sidebar.drag.preview")
			return !ok
		}); err != nil {
			t.Fatalf("drag preview remained after cancellation: %v", err)
		}
		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: end}); err != nil {
			t.Fatal(err)
		}
		drag(note, a)
		activate(b.AutomationID)
		visible(note.AutomationID, true)
		activate(b.AutomationID)
		drag(row("Sidebar nested note"), row("Sidebar B"))
		activate(b.AutomationID)
		visible(note.AutomationID, false)
		activate(b.AutomationID)
		b = row("Sidebar B")
		startRootDrag := woxui.Point{X: b.Bounds.X + b.Bounds.Width/2, Y: b.Bounds.Y + b.Bounds.Height/2}
		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: startRootDrag}); err != nil {
			t.Fatal(err)
		}
		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: startRootDrag.X + 8, Y: startRootDrag.Y}}); err != nil {
			t.Fatal(err)
		}
		var tree woxui.AccessibilityNode
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			var found bool
			tree, found = automationdriver.Find(snapshot, "notes.sidebar.tree")
			_, rootShown := automationdriver.Find(snapshot, "notes.sidebar.row.true.false.")
			return found && !rootShown
		}); err != nil {
			t.Fatalf("unexpected root row during drag: %v", err)
		}
		rootPoint := woxui.Point{X: tree.Bounds.X + 2, Y: startRootDrag.Y}
		for _, event := range []woxui.PointerEvent{{Kind: woxui.PointerMove, Position: rootPoint}, {Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: rootPoint}} {
			if err := client.Pointer(ctx, event); err != nil {
				t.Fatal(err)
			}
		}
		activate(a.AutomationID)
		visible(b.AutomationID, true)
		activate(a.AutomationID)
		drag(row("Sidebar B"), row("Sidebar A"))
		// Deleting A must promote B, keeping the note accessible inside B.
		a = row("Sidebar A")
		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonSecondary, Position: woxui.Point{X: a.Bounds.X + a.Bounds.Width/2, Y: a.Bounds.Y + a.Bounds.Height/2}}); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			_, ok := automationdriver.Find(s, "notes.organize.menu")
			return ok
		}); err != nil {
			t.Fatal(err)
		}
		menuSnapshot, err = client.Snapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		subgroup, found := automationdriver.Find(menuSnapshot, "notes.organize.new-group")
		if !found || (subgroup.Label != "新建子分组" && subgroup.Label != "New Sub Group") {
			t.Fatalf("unexpected subgroup action: %+v", subgroup)
		}
		if _, found = automationdriver.Find(menuSnapshot, "notes.organize.move"); found {
			t.Fatal("redundant group move action is present")
		}
		if os.Getenv("WOX_SMOKE_ARTIFACT_DIR") != "" {
			if err := client.RequestFrame(ctx); err != nil {
				t.Fatal(err)
			}
			if err := client.Capture(ctx, filepath.Join(os.Getenv("WOX_SMOKE_ARTIFACT_DIR"), "notes-sidebar-menu.png")); err != nil {
				t.Fatal(err)
			}
		}
		if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			_, ok := automationdriver.Find(s, "notes.organize.menu")
			return !ok
		}); err != nil {
			t.Fatal(err)
		}
		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonSecondary, Position: woxui.Point{X: a.Bounds.X + a.Bounds.Width/2, Y: a.Bounds.Y + a.Bounds.Height/2}}); err != nil {
			t.Fatal(err)
		}
		if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			_, ok := automationdriver.Find(s, "notes.organize.menu")
			return ok
		}); err != nil {
			t.Fatal(err)
		}
		for _, kind := range []woxui.PointerEventKind{woxui.PointerDown, woxui.PointerUp} {
			if err := client.Pointer(ctx, woxui.PointerEvent{Kind: kind, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 900, Y: 400}}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			_, ok := automationdriver.Find(s, "notes.organize.menu")
			return !ok
		}); err != nil {
			t.Fatal(err)
		}
		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonSecondary, Position: woxui.Point{X: a.Bounds.X + a.Bounds.Width/2, Y: a.Bounds.Y + a.Bounds.Height/2}}); err != nil {
			t.Fatal(err)
		}
		activate("notes.organize.delete")
		activate("notes.organize.confirm")
		if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			_, exists := automationdriver.Find(s, a.AutomationID)
			return !exists
		}); err != nil {
			t.Fatal(err)
		}
		b = row("Sidebar B")
		note = row("Sidebar nested note")
		activate(b.AutomationID)
		visible(note.AutomationID, false)
		activate(b.AutomationID)
		visible(note.AutomationID, true)
		if err := client.PressKey(ctx, woxui.Key("f"), woxui.KeyModifierControl|woxui.KeyModifierMeta); err != nil {
			t.Fatal(err)
		}
		snapshot, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool { _, ok := automationdriver.Find(s, "notes.find"); return ok })
		if err != nil {
			t.Fatal(err)
		}
		find, _ := automationdriver.Find(snapshot, "notes.find")
		editor, _ := automationdriver.Find(snapshot, "notes.editor")
		if find.Bounds.X < editor.Bounds.X || find.Bounds.X+find.Bounds.Width > editor.Bounds.X+editor.Bounds.Width {
			t.Fatal("find bar escaped the editor area")
		}
		modifier := woxui.KeyModifierControl
		if runtime.GOOS == "darwin" {
			modifier = woxui.KeyModifierMeta
		}
		for _, wantVisible := range []bool{false, true} {
			if err := client.PressKey(ctx, woxui.Key("b"), modifier); err != nil {
				t.Fatal(err)
			}
			if _, err := client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
				_, visible := automationdriver.Find(s, "notes.sidebar.tree")
				return visible == wantVisible
			}); err != nil {
				t.Fatalf("sidebar visibility after shortcut: %v", err)
			}
		}
		waitForEditorValue(t, ctx, client, "Sidebar nested note")
		smoke.AssertNoDiagnostics(t, snapshot)
		captureDir := os.Getenv("WOX_SMOKE_ARTIFACT_DIR")
		if captureDir == "" {
			captureDir = t.TempDir()
		}
		if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: editor.Bounds.X + 20, Y: editor.Bounds.Y + 120}}); err != nil {
			t.Fatal(err)
		}
		if err := client.RequestFrame(ctx); err != nil {
			t.Fatal(err)
		}
		if err := client.Capture(ctx, filepath.Join(captureDir, "notes-sidebar.png")); err != nil {
			t.Fatal(err)
		}
		bounds, err := client.Bounds(ctx)
		if err != nil {
			t.Fatal(err)
		}
		activate("notes.toolbar.sidebar")
		bounds.Width = 460
		if err = client.SetBounds(ctx, bounds); err != nil {
			t.Fatal(err)
		}
		activate("notes.toolbar.sidebar")
		if _, err = client.WaitForWindowState(ctx, "notes", func(state automationdriver.WindowState) bool { return state.Exists && state.Visible }); err != nil {
			t.Fatal(err)
		}
		bounds, err = client.Bounds(ctx)
		if err != nil || bounds.Width < 716 {
			t.Fatalf("sidebar did not expand minimum width: %+v %v", bounds, err)
		}
		if _, err = client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			n, ok := automationdriver.Find(s, "notes.sidebar.tree")
			return ok && n.Bounds.Y > 40
		}); err != nil {
			t.Fatal(err)
		}
		if err := client.RequestFrame(ctx); err != nil {
			t.Fatal(err)
		}
		if err := client.Capture(ctx, filepath.Join(captureDir, "notes-sidebar-narrow.png")); err != nil {
			t.Fatal(err)
		}
		activate(row("Sidebar nested note").AutomationID)
		if _, err = client.WaitFor(ctx, func(s woxwidget.AutomationSnapshot) bool {
			_, ok := automationdriver.Find(s, "notes.sidebar.tree")
			return ok
		}); err != nil {
			t.Fatalf("sidebar disappeared after selecting a note: %v", err)
		}
		activate("notes.toolbar.sidebar")
		bounds.Width = 460
		if err = client.SetBounds(ctx, bounds); err != nil {
			t.Fatal(err)
		}
		bounds, err = client.Bounds(ctx)
		if err != nil || bounds.Width > 460 {
			t.Fatalf("closed sidebar did not restore minimum width: %+v %v", bounds, err)
		}
		activate("notes.toolbar.sidebar")
		row("Sidebar nested note")
	})
}
