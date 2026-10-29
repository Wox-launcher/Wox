package launcher

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"wox/common"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// sidebarTestController builds a real controller without native windows.
func sidebarTestController(t *testing.T) *notesWindowController {
	t.Helper()
	app := bindTestApp(&App{services: &notesWindowTestServices{}, appearance: appearanceController{palette: defaultPalette()}})
	c := newNotesWindowController(&app.notes, common.NoteRecord{ID: "note", Document: common.NoteDocument{Blocks: []common.NoteBlock{{Text: "content"}}}})
	c.sidebar.tree = common.NotesTree{Groups: []common.NoteGroup{{ID: "a", Name: "A"}, {ID: "b", Name: "B", ParentID: "a"}}, Placements: []common.NotePlacement{{NoteID: "note", ParentID: "b"}}}
	c.sidebar.notes = []common.NoteSummary{{ID: "note", Title: "Note", PinnedAt: 1}, {ID: "root", Title: "Root"}}
	return c
}

func TestNotesSidebarHierarchyAndPinnedShortcut(t *testing.T) {
	c := sidebarTestController(t)
	c.sidebar.expanded["a"], c.sidebar.expanded["b"] = true, true
	rows := c.sidebarRows()
	count := 0
	for _, row := range rows {
		if row.ID == "note" {
			count++
			if !row.Shortcut && (row.Depth != 2 || row.ParentID != "b") {
				t.Fatalf("wrong nesting: %+v", row)
			}
		}
	}
	if count != 2 {
		t.Fatal("pinned shortcut must retain the original tree entry")
	}
	c.sidebar.expanded["a"] = false
	for _, row := range c.sidebarRows() {
		if row.ID == "b" || row.ID == "note" && !row.Shortcut {
			t.Fatal("collapsed descendants still visible")
		}
	}
	c.sidebar.tree.Groups[0].ParentID = "b"
	rows = c.sidebarRows()
	visible := map[string]bool{}
	for _, row := range rows {
		visible[row.ID] = true
	}
	if !visible["a"] || !visible["b"] || !visible["note"] {
		t.Fatal("remote cycle hid notes")
	}
}

func TestNotesSidebarHidesRootHeadingWithoutPinnedNotes(t *testing.T) {
	c := sidebarTestController(t)
	c.sidebar.notes[0].PinnedAt = 0
	rows := c.sidebarRows()
	if len(rows) != 2 || rows[0].ID != "a" || rows[1].ID != "root" || rows[0].Depth != 0 {
		t.Fatalf("root items should appear directly: %+v", rows)
	}
	c.sidebar.notes[0].PinnedAt = 1
	rows = c.sidebarRows()
	if len(rows) < 3 || !rows[0].Section || !rows[1].Shortcut || !rows[2].Group || rows[2].ID != "" {
		t.Fatalf("root heading should separate pinned notes from the tree: %+v", rows)
	}
}

func TestNotesSidebarRootDropInBlankSpace(t *testing.T) {
	for _, blankBelow := range []bool{false, true} {
		for _, group := range []bool{false, true} {
			c := sidebarTestController(t)
			c.sidebar.notes[0].PinnedAt = 0
			c.sidebar.expanded["a"], c.sidebar.expanded["b"] = true, true
			c.lastFrame.Height = 600
			services := &sidebarTreeTestServices{notesWindowTestServices: &notesWindowTestServices{}, tree: c.sidebar.tree}
			c.owner.services = services
			c.sidebar.dragging, c.sidebar.dragMoved = true, true
			sourceID := "note"
			if group {
				sourceID = "b"
			}
			c.sidebar.dragSource = launcherview.NotesSidebarRow{ID: sourceID, ParentID: "b", Group: group}
			rows := c.sidebarRows()
			for _, row := range rows {
				if row.Group && row.ID == "" {
					t.Fatal("dragging must not add an All notes row")
				}
			}
			c.sidebar.dragViewportX, c.sidebar.dragViewportY = 2, launcherview.NotesSidebarRowHeight/2
			if blankBelow {
				c.sidebar.dragViewportX = 100
				c.sidebar.dragViewportY = (float32(len(rows)) + .5) * launcherview.NotesSidebarRowHeight
			}
			c.resolveSidebarDrop()
			if !c.sidebar.dropRoot {
				t.Fatalf("blank area (below=%v) did not select the root", blankBelow)
			}
			c.finishSidebarDrag()
			if services.change.ID != sourceID || services.change.ParentID != "" || services.change.Action != "move" || services.change.Group != group {
				t.Fatalf("item was not moved to the root: %+v", services.change)
			}
		}
	}
}

func TestNotesSidebarDragToOuterGutter(t *testing.T) {
	c := sidebarTestController(t)
	c.sidebar.notes[0].PinnedAt = 0
	c.sidebar.expanded["a"], c.sidebar.expanded["b"] = true, true
	c.lastFrame.Height = 400 + 16 + launcherview.NotesToolbarHeight
	services := &sidebarTreeTestServices{notesWindowTestServices: &notesWindowTestServices{}, tree: c.sidebar.tree}
	c.owner.services = services
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return c.buildSidebar(woxui.Size{Width: 240, Height: 400}, defaultPalette().componentTheme())
	})
	c.host = host
	defer host.Dispose()
	host.AttachServices(&notesEditorHostServices{})
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 240, Height: 400}, Scale: 1}
	host.Frame(&woxui.DisplayList{}, frame)
	var source, tree woxui.Rect
	for _, node := range host.Snapshot().Tree.Nodes {
		switch node.AutomationID {
		case launcherview.NotesSidebarRowKey(launcherview.NotesSidebarRow{ID: "b", Group: true}):
			source = node.Bounds
		case "notes.sidebar.tree":
			tree = node.Bounds
		}
	}
	if source.Width == 0 || tree.Width == 0 {
		t.Fatal("source row or sidebar tree missing")
	}
	start := woxui.Point{X: source.X + source.Width/2, Y: source.Y + source.Height/2}
	destination := woxui.Point{X: tree.X + 2, Y: start.Y}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: start})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: destination})
	if !c.sidebar.dropRoot {
		t.Fatal("outer gutter did not select the root during pointer drag")
	}
	host.Frame(&woxui.DisplayList{}, frame)
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: destination})
	if services.change.Action != "move" || services.change.ID != "b" || services.change.ParentID != "" || !services.change.Group {
		t.Fatalf("pointer drag did not move the group to root: %+v", services.change)
	}
}

func TestNotesSidebarRootDropRejectsOutsideViewport(t *testing.T) {
	c := sidebarTestController(t)
	c.lastFrame.Height = 600
	c.sidebar.dragging, c.sidebar.dragMoved = true, true
	for _, point := range []woxui.Point{{X: -1, Y: 10}, {X: 300, Y: 10}, {X: 2, Y: -1}, {X: 2, Y: 600}} {
		c.sidebar.dragViewportX, c.sidebar.dragViewportY = point.X, point.Y
		c.resolveSidebarDrop()
		if c.sidebar.dropRoot || c.sidebar.dropIndex >= 0 {
			t.Fatalf("outside point accepted: %+v", point)
		}
	}
}

func TestNotesSidebarBlankHeaderSelectsRootWithoutPinnedNotes(t *testing.T) {
	c := sidebarTestController(t)
	c.sidebar.notes[0].PinnedAt = 0
	c.sidebar.selected, c.sidebar.selectedGroup = "b", true
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return c.buildSidebar(woxui.Size{Width: 240, Height: 400}, defaultPalette().componentTheme())
	})
	defer host.Dispose()
	host.AttachServices(&notesEditorHostServices{})
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 240, Height: 400}, Scale: 1})
	var bounds woxui.Rect
	for _, node := range host.Snapshot().Tree.Nodes {
		if node.AutomationID == "notes.sidebar.root" {
			bounds = node.Bounds
		}
	}
	if bounds.Width == 0 {
		t.Fatal("root selection is unavailable")
	}
	point := woxui.Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
	if c.sidebarParent() != "" {
		t.Fatal("selecting the blank header did not select the root")
	}
}

func TestNotesSidebarToggleAllGroupsPersists(t *testing.T) {
	c := sidebarTestController(t)
	c.sidebar.tree.Groups = append(c.sidebar.tree.Groups, common.NoteGroup{ID: "deleted", Deleted: true})
	c.sidebar.expanded["a"] = true
	c.toggleAllSidebarGroups()
	if c.sidebar.expanded["a"] || c.sidebar.expanded["b"] || c.sidebar.expanded["deleted"] {
		t.Fatal("partially expanded tree should collapse every live group")
	}
	c.toggleAllSidebarGroups()
	if !c.sidebar.expanded["a"] || !c.sidebar.expanded["b"] || c.sidebar.expanded["deleted"] {
		t.Fatal("collapsed tree should expand every live group, including nested groups")
	}
	reopened := newNotesWindowController(c.owner, c.record)
	if !reopened.sidebar.expanded["a"] || !reopened.sidebar.expanded["b"] || reopened.sidebar.expanded["deleted"] {
		t.Fatal("bulk expansion was not persisted locally")
	}
}

func TestNotesSidebarToggleAllButton(t *testing.T) {
	c := sidebarTestController(t)
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return c.buildSidebar(woxui.Size{Width: 240, Height: 400}, defaultPalette().componentTheme())
	})
	defer host.Dispose()
	host.AttachServices(&notesEditorHostServices{})
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 240, Height: 400}, Scale: 1}
	for _, wantExpanded := range []bool{true, false} {
		host.Frame(&woxui.DisplayList{}, frame)
		var bounds woxui.Rect
		for _, node := range host.Snapshot().Tree.Nodes {
			if node.AutomationID == "notes.sidebar.toggle-all" {
				bounds = node.Bounds
			}
		}
		if bounds.Width == 0 {
			t.Fatal("bulk toggle button missing")
		}
		point := woxui.Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
		if c.sidebar.expanded["a"] != wantExpanded || c.sidebar.expanded["b"] != wantExpanded {
			t.Fatalf("bulk toggle did not set expansion to %v", wantExpanded)
		}
	}
}

func TestNotesSidebarShortcut(t *testing.T) {
	c := sidebarTestController(t)
	modifier := woxui.KeyModifierControl
	if primaryHotkey("b") == "command+b" {
		modifier = woxui.KeyModifierMeta
	}
	event := woxui.KeyEvent{Key: "b", Modifiers: modifier, Down: true}
	if !c.onSidebarShortcut(event) || !c.sidebar.open {
		t.Fatal("primary+B must open the sidebar")
	}
	event.Repeat = true
	if !c.onSidebarShortcut(event) || !c.sidebar.open {
		t.Fatal("key repeat must not toggle the sidebar again")
	}
	event.Repeat = false
	if !c.onSidebarShortcut(event) || c.sidebar.open {
		t.Fatal("a second primary+B must close the sidebar")
	}
	event.Modifiers |= woxui.KeyModifierShift
	if c.onSidebarShortcut(event) || c.sidebar.open {
		t.Fatal("a modified shortcut must not toggle the sidebar")
	}
}

func TestNotesSearchHintIncludesPlatformShortcut(t *testing.T) {
	c := sidebarTestController(t)
	want := "(" + strings.Join(formatHotkeyLabels(primaryHotkey("p")), "+") + ")"
	if !strings.HasSuffix(c.searchHint(), want) {
		t.Fatalf("search hint %q does not end in %q", c.searchHint(), want)
	}
}

func TestNotesToolbarSidebarIconTracksVisibility(t *testing.T) {
	c := sidebarTestController(t)
	var icons [2]*woxui.Image
	for index, open := range []bool{false, true} {
		c.sidebar.open = open
		bar := c.buildToolbar(800, true, defaultPalette().componentTheme()).(woxwidget.Container).Child.(woxwidget.Stack)
		button := bar.Children[len(bar.Children)-1].Child.(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
		icons[index] = button.Icon.(woxwidget.Image).Source
	}
	if icons[0] == nil || icons[1] == nil || icons[0] == icons[1] {
		t.Fatal("sidebar icon did not change with its visibility")
	}
}

func TestNotesSidebarDragRejectsDescendantAndCancels(t *testing.T) {
	c := sidebarTestController(t)
	c.sidebar.expanded["a"], c.sidebar.expanded["b"] = true, true
	rows := c.sidebarRows()
	var source launcherview.NotesSidebarRow
	index := 0
	for i, row := range rows {
		if row.ID == "a" {
			source = row
		}
		if row.ID == "b" {
			index = i
		}
	}
	c.sidebar.dragging, c.sidebar.dragMoved = true, true
	c.sidebar.dragSource = source
	c.sidebar.dragViewportX = 80
	c.sidebar.dragViewportY = (float32(index) + .5) * launcherview.NotesSidebarRowHeight
	c.resolveSidebarDrop()
	if c.sidebar.dropIndex != -1 {
		t.Fatal("descendant accepted as drop target")
	}
	c.cancelSidebarDrag()
	if c.sidebar.dragging || c.sidebar.dropIndex != -1 {
		t.Fatal("drag not cancelled")
	}
}

func TestNotesSidebarLayoutAndPointerPan(t *testing.T) {
	for _, density := range []float32{.9, 1, 1.1} {
		c := sidebarTestController(t)
		c.sidebar.expanded["a"] = true
		c.sidebar.open = true
		theme := defaultPalette().componentTheme()
		theme.Controls.DensityScale = density
		rows := c.sidebarRows()
		selected, started, moved, ended, tooltips := 0, 0, 0, 0, 0
		dropIndex := -1
		host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
			return launcherview.NotesSidebar(launcherview.NotesSidebarProps{
				Width: theme.Controls.Scaled(240), Height: 400, Theme: theme, Rows: rows, Scroll: c.sidebar.scroll, Horizontal: c.sidebar.horizontal, DropIndex: dropIndex, DropInto: true,
				Title: "Notes", SearchLabel: "Search", NewLabel: "New", GroupLabel: "Group", MoreLabel: "More", OnSelect: func(int) { selected++ }, OnToggle: func(int) {}, OnMenu: func(int) {}, OnKey: func(int, woxui.KeyEvent) bool { return false }, OnHover: func(bool, string, woxui.Rect) { tooltips++ }, OnDragStart: func(int, woxui.Point) { started++ }, OnDragUpdate: func(woxui.Point) { moved++ }, OnDragEnd: func() { ended++ },
			})
		})
		host.AttachServices(&notesEditorHostServices{})
		host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: theme.Controls.Scaled(240), Height: 400}, Scale: 1})
		if diagnostics := host.Snapshot().Diagnostics; len(diagnostics) > 0 {
			t.Fatalf("density %v: %v", density, diagnostics)
		}
		// A group's label, rather than its disclosure/menu buttons, owns the drag gesture.
		var point woxui.Point
		for _, node := range host.Snapshot().Tree.Nodes {
			if node.AutomationID == launcherview.NotesSidebarRowKey(launcherview.NotesSidebarRow{ID: "a", Group: true}) {
				point = woxui.Point{X: node.Bounds.X + theme.Controls.Scaled(50), Y: node.Bounds.Y + node.Bounds.Height/2}
			}
		}
		if point == (woxui.Point{}) {
			t.Fatal("group label missing")
		}
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
		point.Y += 15
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: point})
		dropIndex = 3
		host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: theme.Controls.Scaled(240), Height: 400}, Scale: 1})
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
		if started != 1 || moved == 0 || ended != 1 || selected != 0 {
			t.Fatalf("pan not delivered: %d/%d/%d/%d", started, moved, ended, selected)
		}
		if tooltips != 0 {
			t.Fatal("tree item displayed a tooltip")
		}
		host.Dispose()
	}
}

func TestNotesSidebarSearchReplacesTree(t *testing.T) {
	for _, density := range []float32{.9, 1, 1.1} {
		query := ""
		theme := defaultPalette().componentTheme()
		theme.Controls.DensityScale = density
		host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
			return launcherview.NotesSidebar(launcherview.NotesSidebarProps{Width: theme.Controls.Scaled(240), Height: 400, Theme: theme, Title: "Notes", SearchLabel: "Search", SearchValue: query, EmptyLabel: "No results", SearchItems: []launcherview.NotesSearchItem{{ID: "match", Label: "Matching note", OnTap: func() {}}}, Rows: []launcherview.NotesSidebarRow{{ID: "tree", Title: "Tree note"}}, NewLabel: "New", GroupLabel: "Group", MoreLabel: "More", OnSelect: func(int) {}, OnKey: func(int, woxui.KeyEvent) bool { return false }, OnHover: func(bool, string, woxui.Rect) {}})
		})
		host.AttachServices(&notesEditorHostServices{})
		frame := woxui.FrameInfo{Size: woxui.Size{Width: theme.Controls.Scaled(240), Height: 400}, Scale: 1}
		for _, value := range []string{"", "match", ""} {
			query = value
			host.Frame(&woxui.DisplayList{}, frame)
			found := map[string]bool{}
			for _, node := range host.Snapshot().Tree.Nodes {
				found[node.AutomationID] = true
			}
			if !found["notes.search"] || found["notes.search.match"] != (value != "") || found["notes.sidebar.tree"] != (value == "") {
				t.Fatalf("density %v query %q: unexpected sidebar nodes %#v", density, value, found)
			}
		}
		host.Dispose()
	}
}

func TestNotesSidebarClearSearch(t *testing.T) {
	for _, density := range []float32{.9, 1, 1.1} {
		c := sidebarTestController(t)
		c.searchEditor.SetText("find a note", false)
		c.searchIndex = 2
		theme := defaultPalette().componentTheme()
		theme.Controls.DensityScale = density
		host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
			return c.buildSidebar(woxui.Size{Width: 240, Height: 400}, theme)
		})
		host.AttachServices(&notesEditorHostServices{})
		frame := woxui.FrameInfo{Size: woxui.Size{Width: theme.Controls.Scaled(240), Height: 400}, Scale: 1}
		host.Frame(&woxui.DisplayList{}, frame)
		var clear woxui.Rect
		for _, node := range host.Snapshot().Tree.Nodes {
			if node.AutomationID == "notes.search.clear" {
				clear = node.Bounds
			}
		}
		if clear.Width == 0 {
			t.Fatalf("density %v: clear button missing", density)
		}
		point := woxui.Point{X: clear.X + clear.Width/2, Y: clear.Y + clear.Height/2}
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
		host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
		if c.searchEditor.Text() != "" || c.searchIndex != 0 || !c.searchOpen {
			t.Fatalf("density %v: search was not cleared", density)
		}
		host.Frame(&woxui.DisplayList{}, frame)
		treeVisible := false
		for _, node := range host.Snapshot().Tree.Nodes {
			if node.AutomationID == "notes.search.clear" || node.AutomationID == "notes.search.results" {
				t.Fatalf("density %v: stale search UI remains: %s", density, node.AutomationID)
			}
			if node.AutomationID == "notes.sidebar.tree" {
				treeVisible = true
			}
		}
		if !treeVisible {
			t.Fatalf("density %v: notes tree did not return", density)
		}
		host.Dispose()
	}
}

func TestNotesSidebarDragDoesNotHighlightHoveredRow(t *testing.T) {
	theme := defaultPalette().componentTheme()
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return launcherview.NotesSidebar(launcherview.NotesSidebarProps{Width: 240, Height: 400, Theme: theme, Title: "Notes", SearchLabel: "Search", NewLabel: "New", GroupLabel: "Group", MoreLabel: "More", Dragging: true, DropIndex: -1, Rows: []launcherview.NotesSidebarRow{{ID: "note", Title: "Note"}}, OnSelect: func(int) {}, OnKey: func(int, woxui.KeyEvent) bool { return false }, OnHover: func(bool, string, woxui.Rect) {}})
	})
	host.AttachServices(&notesEditorHostServices{})
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 240, Height: 400}, Scale: 1}
	host.Frame(&woxui.DisplayList{}, frame)
	var bounds woxui.Rect
	for _, node := range host.Snapshot().Tree.Nodes {
		if node.AutomationID == launcherview.NotesSidebarRowKey(launcherview.NotesSidebarRow{ID: "note"}) {
			bounds = node.Bounds
		}
	}
	if bounds.Width == 0 {
		t.Fatal("note row missing")
	}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: 300, Y: 10}})
	withoutHover := &woxui.DisplayList{}
	host.Frame(withoutHover, frame)
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: woxui.Point{X: bounds.X + 50, Y: bounds.Y + bounds.Height/2}})
	withHover := &woxui.DisplayList{}
	host.Frame(withHover, frame)
	if err := withoutHover.Compare(withHover); err != nil {
		t.Fatalf("hover changed sidebar appearance during drag: %v", err)
	}
	host.Dispose()
}

// sidebarTreeTestServices lets remote snapshots change independently of the active document.
type sidebarTreeTestServices struct {
	*notesWindowTestServices
	tree   common.NotesTree
	items  []common.NoteSummary
	change common.NotesTreeChange
}

func (s *sidebarTreeTestServices) NotesTree(context.Context) (common.NotesTree, error) {
	return s.tree, nil
}
func (s *sidebarTreeTestServices) NotesList(context.Context, string, bool) ([]common.NoteSummary, error) {
	return s.items, nil
}
func (s *sidebarTreeTestServices) NotesOrganize(_ context.Context, change common.NotesTreeChange) (common.NotesTree, error) {
	s.change = change
	return s.tree, nil
}

func TestNotesSidebarGroupDeleteRequiresConfirmation(t *testing.T) {
	c := sidebarTestController(t)
	services := &sidebarTreeTestServices{notesWindowTestServices: &notesWindowTestServices{}, tree: c.sidebar.tree}
	c.owner.services = services
	target := launcherview.NotesSidebarRow{ID: "a", Title: "A", Group: true}
	c.openSidebarDialog("delete", target)
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return c.sidebarDialog(woxui.Size{Width: 700, Height: 400}, defaultPalette().componentTheme())
	})
	defer host.Dispose()
	host.AttachServices(&notesEditorHostServices{})
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 700, Height: 400}, Scale: 1}
	host.Frame(&woxui.DisplayList{}, frame)
	if services.change.Action != "" {
		t.Fatal("opening confirmation deleted the group")
	}
	var bounds woxui.Rect
	for _, node := range host.Snapshot().Tree.Nodes {
		if node.AutomationID == "notes.organize.confirm" {
			bounds = node.Bounds
		}
	}
	if bounds.Width == 0 {
		t.Fatal("confirmation button missing")
	}
	point := woxui.Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
	if services.change.Action != "delete" || services.change.ID != "a" || !services.change.Group || c.sidebar.dialog != "" {
		t.Fatalf("confirmation did not delete the selected group: %+v", services.change)
	}
}

func TestNotesSidebarSaveFailureAndSwitch(t *testing.T) {
	c := sidebarTestController(t)
	services := &sidebarTreeTestServices{notesWindowTestServices: &notesWindowTestServices{saveErr: errors.New("disk failure"), records: map[string]common.NoteRecord{"root": {ID: "root", Document: common.NoteDocument{Blocks: []common.NoteBlock{{Text: "destination"}}}}}}, items: c.sidebar.notes}
	c.owner.services = services
	c.dirty = true
	c.sidebar.open = true
	c.selectSidebarRow(launcherview.NotesSidebarRow{ID: "root"})
	if c.record.ID != "note" || !c.dirty || c.errorText == "" || !c.sidebar.open {
		t.Fatal("failed save changed the document or closed the sidebar")
	}
	services.saveErr = nil
	c.selectSidebarRow(launcherview.NotesSidebarRow{ID: "root"})
	if c.record.ID != "root" || c.dirty || !c.sidebar.open {
		t.Fatal("successful switch did not keep the sidebar open")
	}
}

func TestNotesSidebarRemoteRefreshRetainsEditsAndManualOrder(t *testing.T) {
	c := sidebarTestController(t)
	services := &sidebarTreeTestServices{notesWindowTestServices: &notesWindowTestServices{}, tree: common.NotesTree{Placements: []common.NotePlacement{{NoteID: "a", Order: 0}, {NoteID: "b", Order: 0}}}, items: []common.NoteSummary{{ID: "a", UpdatedAt: 1}, {ID: "b", UpdatedAt: 100}}}
	c.owner.services = services
	c.dirty = true
	c.refresh("")
	rows := c.sidebarRows()
	if len(rows) != 2 || rows[0].ID != "a" || rows[1].ID != "b" || !c.dirty || c.document.Blocks[0].Text != "content" {
		t.Fatal("remote organization refresh reordered tied placements or replaced local edits")
	}
	services.tree.Groups = []common.NoteGroup{{ID: "parent", Name: "Remote"}}
	services.tree.Placements[0].ParentID = "parent"
	c.sidebar.expanded["parent"] = true
	c.refresh("")
	rows = c.sidebarRows()
	if rows[0].ID != "parent" || rows[1].ID != "a" || rows[1].Depth != 1 {
		t.Fatal("remote move not reflected in tree")
	}
	services.tree.Groups[0].Deleted = true
	c.refresh("")
	rows = c.sidebarRows()
	if len(rows) != 2 || rows[0].ID != "a" {
		t.Fatal("remote deletion hid a late child")
	}
}

func TestNotesSidebarDraftPlacementAndMinimumWidth(t *testing.T) {
	c := sidebarTestController(t)
	services := &sidebarTreeTestServices{notesWindowTestServices: &notesWindowTestServices{}, tree: c.sidebar.tree}
	c.owner.services = services
	parent := "b"
	c.sidebar.pendingParent = &parent
	if err := c.placeSidebarDraft(); err != nil {
		t.Fatal(err)
	}
	if services.change.ParentID != "b" || !services.change.First {
		t.Fatal("collapsed group's new note was not inserted first")
	}
	c.sidebar.open = true
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget { return woxwidget.Container{} })
	defer host.Dispose()
	c.frame(host, &woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 900, Height: 600}, Scale: 1.5})
	if c.noteContentWidth() != 644 || c.sidebarMinSize().Width != 716 {
		t.Fatal("sidebar did not reserve the editor minimum width")
	}
}

func TestNotesSidebarMinimumWidthScalesWithDensity(t *testing.T) {
	for _, density := range []float32{.9, 1, 1.1} {
		c := sidebarTestController(t)
		controls := c.controlTheme()
		controls.DensityScale = density
		c.controlTheme = func() woxcomponent.ControlTheme { return controls }
		if c.sidebarMinSize().Width != notesMinimumWidth {
			t.Fatal("closed sidebar changed minimum width")
		}
		c.sidebar.open = true
		if c.sidebarMinSize().Width != notesMinimumWidth+controls.Scaled(256) {
			t.Fatal("sidebar minimum did not scale with UI density")
		}
	}
}

func TestNotesSidebarDeepTreeAndLocalPreferences(t *testing.T) {
	c := sidebarTestController(t)
	c.sidebar.tree.Groups = nil
	parent := ""
	for i := 0; i < 24; i++ {
		id := fmt.Sprintf("level-%d", i)
		c.sidebar.tree.Groups = append(c.sidebar.tree.Groups, common.NoteGroup{ID: id, ParentID: parent, Name: id})
		c.toggleSidebarGroup(id)
		parent = id
	}
	c.sidebar.tree.Placements[0].ParentID = parent
	c.sidebar.notes[0].Title = strings.Repeat("long note title ", 20)
	c.sidebar.notes[0].PinnedAt = 0
	c.sidebar.open = true
	if err := c.owner.services.NotesSetLocal(context.Background(), "sidebar.open", "1"); err != nil {
		t.Fatal(err)
	}
	theme := defaultPalette().componentTheme()
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return c.buildSidebar(woxui.Size{Width: 240, Height: 400}, theme)
	})
	defer host.Dispose()
	host.AttachServices(&notesEditorHostServices{})
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 240, Height: 400}, Scale: 2}
	host.Frame(&woxui.DisplayList{}, frame)
	if c.sidebar.horizontal.MaxOffset() <= 0 || c.sidebar.scroll.MaxOffset() <= 0 {
		t.Fatal("deep tree cannot scroll in both directions")
	}
	c.sidebar.horizontal.JumpTo(c.sidebar.horizontal.MaxOffset())
	c.sidebar.scroll.JumpTo(c.sidebar.scroll.MaxOffset())
	host.Frame(&woxui.DisplayList{}, frame)
	if diagnostics := host.Snapshot().Diagnostics; len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	c.persistSidebarScroll()
	reopened := newNotesWindowController(c.owner, c.record)
	if !reopened.sidebar.open || !reopened.sidebar.expanded[parent] || reopened.sidebar.scroll.Offset() != c.sidebar.scroll.Offset() || reopened.sidebar.horizontal.Offset() != c.sidebar.horizontal.Offset() {
		t.Fatal("reopened window lost local sidebar preferences")
	}
}
