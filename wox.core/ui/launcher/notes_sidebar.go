package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"wox/common"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// notesSidebarState belongs to one UI-thread window; persisted organization belongs to the repository.
type notesSidebarState struct {
	open                bool
	minimumWidth        float32
	tree                common.NotesTree
	notes               []common.NoteSummary
	expanded            map[string]bool
	selected            string
	selectedGroup       bool
	scroll, horizontal  *woxwidget.ScrollController
	dialog              string
	target              launcherview.NotesSidebarRow
	menuAnchor          woxui.Rect
	menuFocusPending    bool
	name                *woxwidget.TextEditingController
	errorText           string
	pendingParent       *string
	dragMoved           bool
	pressPoint          woxui.Point
	dragPointer         woxui.Point
	dragViewportY       float32
	dragViewportX       float32
	dragging            bool
	dragSource          launcherview.NotesSidebarRow
	dropIndex           int
	dropInto, dropAfter bool
	dropRoot            bool
	hoverGroup          string
	hoverSince          time.Time
	dragGeneration      uint64
}

// initSidebar restores device-only presentation preferences without rewriting old notes.
func (c *notesWindowController) initSidebar() {
	s := &c.sidebar
	s.expanded = map[string]bool{}
	s.scroll = woxwidget.NewScrollController(0)
	s.horizontal = woxwidget.NewScrollController(0)
	s.name = woxwidget.NewTextEditingController("")
	s.dropIndex = -1
	if c.owner == nil || c.owner.services == nil {
		return
	}
	value, _ := c.owner.services.NotesGetLocal(context.Background(), "sidebar.open")
	s.open = value == "1"
	value, _ = c.owner.services.NotesGetLocal(context.Background(), "sidebar.scroll")
	if offset, err := strconv.ParseFloat(value, 32); err == nil {
		s.scroll = woxwidget.NewScrollController(float32(offset))
	}
	value, _ = c.owner.services.NotesGetLocal(context.Background(), "sidebar.horizontal")
	if offset, err := strconv.ParseFloat(value, 32); err == nil {
		s.horizontal = woxwidget.NewScrollController(float32(offset))
	}
	value, _ = c.owner.services.NotesGetLocal(context.Background(), "sidebar.expanded")
	_ = json.Unmarshal([]byte(value), &s.expanded)
	if s.expanded == nil {
		s.expanded = map[string]bool{}
	}
}

func (c *notesWindowController) sidebarLabel(key string) string {
	return c.owner.language.translate("i18n:notes_" + key)
}

func (c *notesWindowController) searchHint() string {
	return c.sidebarLabel("search_placeholder") + " (" + strings.Join(formatHotkeyLabels(primaryHotkey("p")), "+") + ")"
}

// reloadSidebar also refreshes closed sidebars so reopening never shows stale organization.
func (c *notesWindowController) reloadSidebar() error {
	tree, err := c.owner.services.NotesTree(context.Background())
	if err != nil {
		return err
	}
	items, err := c.owner.services.NotesList(context.Background(), "", false)
	if err != nil {
		return err
	}
	c.sidebar.tree, c.sidebar.notes = tree, items
	return nil
}

// onSidebarShortcut handles the window shortcut before focused editors consume it.
func (c *notesWindowController) onSidebarShortcut(event woxui.KeyEvent) bool {
	if !event.Down || event.Composing || !hotkeyMatches(primaryHotkey("b"), event) {
		return false
	}
	if !event.Repeat {
		c.toggleSidebar()
	}
	return true
}

// toggleSidebar changes the remembered preference for explicit user actions.
func (c *notesWindowController) toggleSidebar() {
	s := &c.sidebar
	if s.dragging {
		c.cancelSidebarDrag()
	}
	s.open = !s.open
	if !s.open {
		c.searchOpen = false
	}
	c.syncSidebarMinSize()
	value := "0"
	if s.open {
		value = "1"
	}
	if err := c.owner.services.NotesSetLocal(context.Background(), "sidebar.open", value); err != nil {
		c.fail(err)
	}
	if s.open {
		if err := c.reloadSidebar(); err != nil {
			c.fail(err)
		}
	}
	c.invalidate()
}

// toggleSidebarGroup persists disclosure state without changing organization.
func (c *notesWindowController) toggleSidebarGroup(id string) {
	if id == "" {
		return
	}
	c.sidebar.expanded[id] = !c.sidebar.expanded[id]
	value, _ := json.Marshal(c.sidebar.expanded)
	if err := c.owner.services.NotesSetLocal(context.Background(), "sidebar.expanded", string(value)); err != nil {
		c.fail(err)
	}
	c.invalidate()
}

// toggleAllSidebarGroups changes every live group in one local preference write.
func (c *notesWindowController) toggleAllSidebarGroups() {
	anyExpanded := false
	for _, group := range c.sidebar.tree.Groups {
		if !group.Deleted && c.sidebar.expanded[group.ID] {
			anyExpanded = true
			break
		}
	}
	for _, group := range c.sidebar.tree.Groups {
		if !group.Deleted {
			c.sidebar.expanded[group.ID] = !anyExpanded
		}
	}
	value, _ := json.Marshal(c.sidebar.expanded)
	if err := c.owner.services.NotesSetLocal(context.Background(), "sidebar.expanded", string(value)); err != nil {
		c.fail(err)
	}
	c.invalidate()
}

// sidebarRows flattens the normalized tree while retaining the repository's sibling order.
func (c *notesWindowController) sidebarRows() []launcherview.NotesSidebarRow {
	s := &c.sidebar
	groups := map[string]common.NoteGroup{}
	for _, g := range s.tree.Groups {
		groups[g.ID] = g
	}
	positions := map[string]common.NotePlacement{}
	for _, p := range s.tree.Placements {
		positions[p.NoteID] = p
	}
	children := map[string][]common.NoteGroup{}
	notes := map[string][]common.NoteSummary{}
	for _, g := range s.tree.Groups {
		if !g.Deleted {
			parent := common.ResolveNoteParent(g.ParentID, groups)
			children[parent] = append(children[parent], g)
		}
	}
	for _, n := range s.notes {
		if n.DeletedAt == 0 {
			parent := common.ResolveNoteParent(positions[n.ID].ParentID, groups)
			notes[parent] = append(notes[parent], n)
		}
	}
	for parent := range children {
		sort.Slice(children[parent], func(i, j int) bool {
			a, b := children[parent][i], children[parent][j]
			if a.Order != b.Order {
				return a.Order < b.Order
			}
			return a.ID < b.ID
		})
	}
	for parent := range notes {
		sort.SliceStable(notes[parent], func(i, j int) bool {
			a, b := notes[parent][i], notes[parent][j]
			pa, oka := positions[a.ID]
			pb, okb := positions[b.ID]
			if oka != okb {
				return oka
			}
			if oka && pa.Order != pb.Order {
				return pa.Order < pb.Order
			}
			if !oka && a.UpdatedAt != b.UpdatedAt {
				return a.UpdatedAt > b.UpdatedAt
			}
			return a.ID < b.ID
		})
	}
	var rows []launcherview.NotesSidebarRow
	for _, n := range s.notes {
		if n.PinnedAt > 0 && n.DeletedAt == 0 {
			if len(rows) == 0 {
				rows = append(rows, launcherview.NotesSidebarRow{Title: c.sidebarLabel("pinned"), Section: true})
			}
			rows = append(rows, launcherview.NotesSidebarRow{ID: n.ID, Title: n.Title, Shortcut: true, Selected: n.ID == c.record.ID})
		}
	}
	hasPinned := len(rows) > 0
	if hasPinned {
		rows = append(rows, launcherview.NotesSidebarRow{Title: c.sidebarLabel("all_notes"), Group: true, Expanded: true, Selected: s.selectedGroup && s.selected == ""})
	}
	var appendChildren func(string, int)
	appendChildren = func(parent string, depth int) {
		for _, g := range children[parent] {
			rows = append(rows, launcherview.NotesSidebarRow{ID: g.ID, Title: g.Name, ParentID: parent, Group: true, Depth: depth, Expanded: s.expanded[g.ID], Selected: s.selectedGroup && s.selected == g.ID})
			if s.expanded[g.ID] {
				appendChildren(g.ID, depth+1)
			}
		}
		for _, n := range notes[parent] {
			rows = append(rows, launcherview.NotesSidebarRow{ID: n.ID, Title: n.Title, ParentID: parent, Depth: depth, Selected: n.ID == c.record.ID})
		}
	}
	appendChildren("", 0)
	return rows
}

// selectSidebarNote preserves autosave failure handling and the existing one-window-per-note policy.
func (c *notesWindowController) selectSidebarRow(row launcherview.NotesSidebarRow) {
	if row.Section {
		return
	}
	c.sidebar.selected, c.sidebar.selectedGroup = row.ID, row.Group
	if row.Group {
		c.toggleSidebarGroup(row.ID)
		c.invalidate()
		return
	}
	var item common.NoteSummary
	for _, n := range c.sidebar.notes {
		if n.ID == row.ID {
			item = n
			break
		}
	}
	if item.ID == "" {
		return
	}
	if err := c.persistCurrentNote(); err != nil {
		c.fail(err)
		return
	}
	record, err := c.owner.services.NotesGet(context.Background(), item.ID)
	if err != nil {
		c.fail(err)
		return
	}
	if err := c.showNoteInCurrentWindow(record); err != nil {
		c.fail(err)
		return
	}
	c.editorFocus.RequestFocus()
	c.invalidate()
}

// sidebarParent uses the selected group or the active note's current location for new items.
func (c *notesWindowController) sidebarParent() string {
	groups := map[string]common.NoteGroup{}
	for _, g := range c.sidebar.tree.Groups {
		groups[g.ID] = g
	}
	if c.sidebar.selectedGroup {
		return common.ResolveNoteParent(c.sidebar.selected, groups)
	}
	for _, p := range c.sidebar.tree.Placements {
		if p.NoteID == c.record.ID {
			return common.ResolveNoteParent(p.ParentID, groups)
		}
	}
	return ""
}

// newSidebarNote opens an ephemeral draft and defers its placement until content is saved.
func (c *notesWindowController) newSidebarNote() {
	if err := c.persistCurrentNote(); err != nil {
		c.fail(err)
		return
	}
	parent := c.sidebarParent()
	record, err := c.owner.services.NotesCreate(context.Background())
	if err != nil {
		c.fail(err)
		return
	}
	if err := c.showNoteInCurrentWindow(record); err != nil {
		c.fail(err)
		return
	}
	c.sidebar.pendingParent = &parent
	c.editorFocus.RequestFocus()
	c.invalidate()
}

// placeSidebarDraft waits for the first saved content, preserving ephemeral empty drafts.
func (c *notesWindowController) placeSidebarDraft() error {
	if c.sidebar.pendingParent == nil {
		return nil
	}
	groups := map[string]common.NoteGroup{}
	for _, g := range c.sidebar.tree.Groups {
		groups[g.ID] = g
	}
	change := common.NotesTreeChange{Action: "move", ID: c.record.ID, ParentID: common.ResolveNoteParent(*c.sidebar.pendingParent, groups), First: true}
	tree, err := c.owner.services.NotesOrganize(context.Background(), change)
	if err != nil {
		c.errorText = c.sidebarLabel("organize_failed")
		return err
	}
	c.sidebar.tree = tree
	c.sidebar.pendingParent = nil
	return nil
}

// organizeNotes keeps failures in the active dialog and publishes successful tree snapshots.
func (c *notesWindowController) organizeNotes(change common.NotesTreeChange) bool {
	tree, err := c.owner.services.NotesOrganize(context.Background(), change)
	if err != nil {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("organize Notes: %v", err))
		c.sidebar.errorText = c.sidebarLabel("organize_failed")
		c.invalidate()
		return false
	}
	c.sidebar.tree = tree
	c.sidebar.errorText = ""
	c.invalidate()
	return true
}

// sidebarKey navigates the focused tree.
func (c *notesWindowController) sidebarKey(index int, event woxui.KeyEvent) bool {
	if !event.Down || event.Composing {
		return false
	}
	rows := c.sidebarRows()
	if index < 0 || index >= len(rows) {
		return false
	}
	row := rows[index]
	next := index
	switch event.Key {
	case woxui.KeyArrowDown:
		next++
	case woxui.KeyArrowUp:
		next--
	case woxui.KeyArrowRight:
		if row.Group && !row.Expanded {
			c.toggleSidebarGroup(row.ID)
			return true
		}
		next++
	case woxui.KeyArrowLeft:
		if row.Group && row.Expanded && row.ID != "" {
			c.toggleSidebarGroup(row.ID)
			return true
		}
		for i := index - 1; i >= 0; i-- {
			if rows[i].Group && rows[i].ID == row.ParentID {
				next = i
				break
			}
		}
	case woxui.KeyEnter:
		c.selectSidebarRow(row)
		return true
	default:
		return false
	}
	step := 1
	if next < index {
		step = -1
	}
	for next >= 0 && next < len(rows) && rows[next].Section {
		next += step
	}
	if next >= 0 && next < len(rows) && c.host != nil {
		c.host.RequestFocus(woxwidget.Key(launcherview.NotesSidebarRowKey(rows[next])))
		height := c.controlTheme().Scaled(launcherview.NotesSidebarRowHeight)
		c.sidebar.scroll.EnsureVisible(float32(next)*height, float32(next+1)*height)
	}
	return true
}

// buildSidebar supplies immutable row data and stable callbacks to the view.
func (c *notesWindowController) buildSidebar(size woxui.Size, theme woxcomponent.Theme) woxwidget.Widget {
	rows := c.sidebarRows()
	items := c.searchItems()
	results := make([]launcherview.NotesSearchItem, 0, len(items))
	for _, item := range items {
		results = append(results, c.searchRowProps(item))
	}
	s := &c.sidebar
	groupLabel := c.sidebarLabel("group_new")
	if c.sidebarParent() != "" {
		groupLabel = c.sidebarLabel("subgroup_new")
	}
	hasGroups, anyExpanded := false, false
	for _, group := range s.tree.Groups {
		if !group.Deleted {
			hasGroups = true
			anyExpanded = anyExpanded || s.expanded[group.ID]
		}
	}
	allLabel := c.sidebarLabel("expand_all")
	if anyExpanded {
		allLabel = c.sidebarLabel("collapse_all")
	}
	var onRootSelect func()
	if len(rows) == 0 || !rows[0].Section {
		onRootSelect = func() { s.selected, s.selectedGroup = "", true; c.invalidate() }
	}
	return launcherview.NotesSidebar(launcherview.NotesSidebarProps{Width: theme.Controls.Scaled(240), Height: size.Height, Title: c.sidebarLabel("sidebar_root"), RootLabel: c.sidebarLabel("select_root"), Rows: rows, Theme: theme, Scroll: s.scroll, Horizontal: s.horizontal,
		SearchLabel: c.sidebarLabel("search"), SearchHint: c.searchHint(), ClearLabel: c.sidebarLabel("search_clear"), EmptyLabel: c.owner.language.translate("i18n:notes_no_results"), SearchValue: c.searchEditor.Text(), SearchItems: results, SearchIndex: c.searchIndex, SearchEditor: c.searchEditor, SearchFocus: c.searchFocus, SearchFocused: c.searchOpen, Window: c.noteMeasureWindow(),
		OnSearchChanged: func(string) { c.searchOpen = true; _ = c.reloadSummaries(); c.searchIndex = 0; c.invalidate() }, OnSearchKey: c.onSearchKey,
		OnSearchClear: func() {
			c.searchEditor.SetText("", false)
			c.searchOpen = true
			c.searchIndex = 0
			_ = c.reloadSummaries()
			c.searchFocus.RequestFocus()
			c.invalidate()
		},
		NewLabel: c.sidebarLabel("new"), GroupLabel: groupLabel, MoreLabel: c.sidebarLabel("more"), ToggleAllLabel: allLabel, HasGroups: hasGroups, AnyExpanded: anyExpanded,
		OnNew: c.newSidebarNote, OnRootSelect: onRootSelect, OnGroup: func() {
			c.openSidebarDialog("create", launcherview.NotesSidebarRow{ID: c.sidebarParent(), Group: true})
		}, OnToggleAll: c.toggleAllSidebarGroups,
		OnSelect: func(i int) { c.selectSidebarRow(rows[i]) }, OnToggle: func(i int) { c.toggleSidebarGroup(rows[i].ID) }, OnMenu: func(i int) {
			if c.host != nil {
				s.menuAnchor, _ = c.host.BoundsForKey(woxwidget.Key(launcherview.NotesSidebarRowKey(rows[i]) + ".more"))
			}
			c.openSidebarDialog("menu", rows[i])
			s.menuFocusPending = true
		}, OnKey: c.sidebarKey, OnHover: c.updateToolbarTooltip,
		OnDragStart: func(i int, p woxui.Point) {
			if rows[i].ID != "" {
				s.dragging = true
				s.dragMoved = false
				s.pressPoint = p
				s.dragSource = rows[i]
				s.dragGeneration++
				c.tickSidebarDrag(s.dragGeneration)
			}
		},
		OnDragUpdate: c.updateSidebarDrag, OnDragEnd: c.finishSidebarDrag, DropIndex: s.dropIndex, DropInto: s.dropInto, DropAfter: s.dropAfter, DropRoot: s.dropRoot, Dragging: s.dragging && s.dragMoved,
	})
}

// cancelSidebarDrag invalidates delayed hover and scroll callbacks.
func (c *notesWindowController) cancelSidebarDrag() {
	c.sidebar.dragging = false
	c.sidebar.dropIndex = -1
	c.sidebar.dropRoot = false
	c.sidebar.hoverGroup = ""
	c.sidebar.dragGeneration++
	c.invalidate()
}

// updateSidebarDrag maps the captured row-local pointer into the currently scrolled tree.
func (c *notesWindowController) updateSidebarDrag(point woxui.Point) {
	s := &c.sidebar
	if !s.dragging {
		return
	}
	if !s.dragMoved {
		dx, dy := point.X-s.pressPoint.X, point.Y-s.pressPoint.Y
		if dx*dx+dy*dy < 16 {
			return
		}
		s.dragMoved = true
	}
	if c.host != nil {
		if bounds, ok := c.host.BoundsForKey(woxwidget.Key(launcherview.NotesSidebarRowKey(s.dragSource))); ok {
			s.dragPointer = woxui.Point{X: bounds.X + point.X, Y: bounds.Y + point.Y}
		}
	}
	c.invalidate()
	rows := c.sidebarRows()
	height := c.controlTheme().Scaled(launcherview.NotesSidebarRowHeight)
	for index, row := range rows {
		if !row.Shortcut && row.ID == s.dragSource.ID && row.Group == s.dragSource.Group {
			s.dragViewportY = float32(index)*height + point.Y - s.scroll.Offset()
			s.dragViewportX = point.X - s.horizontal.Offset()
			break
		}
	}
	c.resolveSidebarDrop()
}

// resolveSidebarDrop uses viewport coordinates so automatic scrolling does not double-count pointer motion.
func (c *notesWindowController) resolveSidebarDrop() {
	s := &c.sidebar
	rows := c.sidebarRows()
	height := c.controlTheme().Scaled(launcherview.NotesSidebarRowHeight)
	y := s.dragViewportY + s.scroll.Offset()

	index := int(y / height)
	s.dropIndex, s.dropRoot = -1, false
	previousHover := s.hoverGroup
	s.hoverGroup = ""
	viewportHeight := c.lastFrame.Height - c.controlTheme().Scaled(112) - launcherview.NotesToolbarHeight
	if s.dragViewportY < 0 || (c.lastFrame.Height > 0 && s.dragViewportY >= viewportHeight) || s.dragViewportX < 0 || s.dragViewportX > c.controlTheme().Scaled(224) {
		return
	}
	// The outer gutter and unused space below the tree represent the root, even when no root row is visible.
	if s.dragViewportX < c.controlTheme().Scaled(8) || index >= len(rows) {
		s.dropRoot = true
		c.invalidate()
		return
	}
	if y < 0 {
		return
	}
	row := rows[index]
	if row.Section || row.Shortcut || row.ID == s.dragSource.ID {
		return
	}
	fraction := (y - float32(index)*height) / height
	into := row.Group && (fraction > .25 && fraction < .75 || row.ID == "")
	if !into && row.Group != s.dragSource.Group {
		return
	}
	parent := row.ParentID
	if into {
		parent = row.ID
	}
	if s.dragSource.Group {
		groups := map[string]common.NoteGroup{}
		for _, g := range s.tree.Groups {
			groups[g.ID] = g
		}
		seen := map[string]bool{}
		for id := parent; id != ""; id = groups[id].ParentID {
			if id == s.dragSource.ID || seen[id] {
				return
			}
			seen[id] = true
		}
	}
	s.dropIndex, s.dropInto, s.dropAfter = index, into, fraction >= .5
	if into {
		s.hoverGroup = row.ID
		if previousHover != row.ID {
			s.hoverSince = time.Now()
		}
	}
	c.invalidate()
}

// tickSidebarDrag keeps hover expansion and edge scrolling working with a stationary pointer.
func (c *notesWindowController) tickSidebarDrag(generation uint64) {
	time.AfterFunc(60*time.Millisecond, func() {
		_ = c.owner.runOnUI("Notes sidebar drag", func() {
			s := &c.sidebar
			if !s.dragging || s.dragGeneration != generation || c.managed == nil {
				return
			}
			height := c.controlTheme().Scaled(launcherview.NotesSidebarRowHeight)
			viewportY := s.dragViewportY
			viewport := c.lastFrame.Height - c.controlTheme().Scaled(152) - launcherview.NotesToolbarHeight
			if s.dragMoved && viewportY < height {
				s.scroll.ScrollBy(-height / 3)
			} else if s.dragMoved && viewportY > viewport-height {
				s.scroll.ScrollBy(height / 3)
			}
			if s.hoverGroup != "" && !s.expanded[s.hoverGroup] && time.Since(s.hoverSince) > 600*time.Millisecond {
				c.toggleSidebarGroup(s.hoverGroup)
			}
			if s.dragMoved {
				c.resolveSidebarDrop()
			}
			c.tickSidebarDrag(generation)
		})
	})
}

// finishSidebarDrag distinguishes clicks from moves and commits one organization transaction.
func (c *notesWindowController) finishSidebarDrag() {
	s := &c.sidebar
	if !s.dragging {
		return
	}
	rows := c.sidebarRows()
	index := s.dropIndex
	source := s.dragSource
	if !s.dragMoved {
		c.cancelSidebarDrag()
		c.selectSidebarRow(source)
		return
	}
	into, after, root := s.dropInto, s.dropAfter, s.dropRoot
	c.cancelSidebarDrag()
	if root {
		if !c.organizeNotes(common.NotesTreeChange{Action: "move", ID: source.ID, Group: source.Group}) {
			c.fail(fmt.Errorf("%s", s.errorText))
		}
		return
	}
	if index < 0 || index >= len(rows) {
		return
	}
	target := rows[index]
	change := common.NotesTreeChange{Action: "move", ID: source.ID, Group: source.Group, ParentID: target.ParentID}
	if into {
		change.ParentID = target.ID
	} else if !after {
		change.BeforeID = target.ID
	} else {
		for _, row := range rows[index+1:] {
			if !row.Shortcut && !row.Section && row.Group == source.Group && row.ParentID == target.ParentID && row.ID != source.ID {
				change.BeforeID = row.ID
				break
			}
		}
	}
	if !c.organizeNotes(change) {
		c.fail(fmt.Errorf("%s", s.errorText))
	}
}

// openSidebarDialog resets transient input while preserving the selected tree target.
func (c *notesWindowController) openSidebarDialog(kind string, row launcherview.NotesSidebarRow) {
	c.sidebar.dialog, c.sidebar.target, c.sidebar.errorText = kind, row, ""
	value := ""
	if kind == "rename" {
		value = row.Title
	}
	c.sidebar.name.SetText(value, false)
	c.invalidate()
}

func (c *notesWindowController) closeSidebarDialog() { c.sidebar.dialog = ""; c.invalidate() }

// sidebarDialog composes menus, naming, confirmation and destination selection in shared dialog chrome.
func (c *notesWindowController) sidebarDialog(size woxui.Size, theme woxcomponent.Theme) woxwidget.Widget {
	s := &c.sidebar
	if s.dialog == "create" || s.dialog == "rename" {
		return c.sidebarGroupDialog(size, theme)
	}
	if s.dialog == "delete" {
		return c.sidebarDeleteDialog(size, theme)
	}
	scale := theme.Controls.Scaled
	width := min(scale(380), size.Width-24)
	inner := width - 32
	if s.dialog == "menu" {
		width = min(scale(220), size.Width-scale(16))
		inner = width - scale(16)
	}
	var rows []woxwidget.Widget
	action := func(id, label string, fn func()) {
		props := woxcomponent.ButtonProps{ID: "notes.organize." + id, Label: label, Width: inner, Height: scale(32), AlignLeading: true, Theme: theme.Controls, OnTap: fn}
		if s.dialog == "menu" {
			var glyph woxwidget.Widget
			switch id {
			case "new-note", "new-group":
				glyph = woxcomponent.AddGlyph(scale(16), theme.Controls.BodyText)
			case "pin":
				glyph = woxcomponent.PinGlyph(scale(16), theme.Controls.BodyText)
			case "rename":
				glyph = woxcomponent.EditGlyph(scale(16), theme.Controls.BodyText)
			case "delete":
				rows = append(rows, woxwidget.Container{Width: inner, Height: scale(1), Color: theme.Controls.Border})
				glyph = woxcomponent.DeleteGlyph(scale(16), theme.Controls.BodyText)
			}
			if glyph != nil {
				props.Icon = glyph.(woxwidget.Image).Source
			}
			props.IconSize, props.IconGap = scale(16), scale(8)
			props.Padding = woxwidget.Insets{Left: scale(8), Right: scale(8)}
			props.Variant = woxcomponent.ButtonText
		}
		rows = append(rows, woxcomponent.WoxButton(props))
	}
	target := s.target
	switch s.dialog {
	case "menu":
		if target.Group {
			action("new-note", c.sidebarLabel("new"), func() { s.selected, s.selectedGroup = target.ID, true; c.closeSidebarDialog(); c.newSidebarNote() })
			action("new-group", c.sidebarLabel("subgroup_new"), func() { c.openSidebarDialog("create", target) })
			if target.ID != "" {
				action("rename", c.sidebarLabel("group_rename"), func() { c.openSidebarDialog("rename", target) })
			}
		} else {
			pinned := false
			for _, n := range s.notes {
				if n.ID == target.ID {
					pinned = n.PinnedAt > 0
				}
			}
			pinLabel := "sidebar_pin"
			if pinned {
				pinLabel = "sidebar_unpin"
			}
			action("pin", c.sidebarLabel(pinLabel), func() {
				if target.ID == c.record.ID {
					if err := c.flush(); err != nil {
						c.fail(err)
						return
					}
				}
				if _, err := c.owner.services.NotesSetPinned(context.Background(), target.ID, !pinned); err != nil {
					c.fail(err)
					return
				}
				c.closeSidebarDialog()
				c.refresh(target.ID)
			})
		}
		if target.ID != "" {
			action("delete", c.sidebarLabel("sidebar_delete"), func() { c.openSidebarDialog("delete", target) })
		}
	case "move":
		groups := map[string]common.NoteGroup{}
		for _, g := range s.tree.Groups {
			groups[g.ID] = g
		}
		addTarget := func(id, label string) {
			action("destination."+id, label, func() {
				if c.organizeNotes(common.NotesTreeChange{Action: "move", ID: target.ID, Group: target.Group, ParentID: id}) {
					c.closeSidebarDialog()
				}
			})
		}
		addTarget("", c.sidebarLabel("sidebar_root"))
		// Reuse the visible tree; disclosure buttons let the picker explore destinations lazily.
		for _, row := range c.sidebarRows() {
			if !row.Group || row.ID == "" {
				continue
			}
			invalid := false
			seen := map[string]bool{}
			for id := row.ID; id != ""; id = groups[id].ParentID {
				if target.Group && id == target.ID || seen[id] {
					invalid = true
					break
				}
				seen[id] = true
			}
			if invalid {
				continue
			}
			row := row
			rows = append(rows, woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{ID: "notes.destination.toggle." + row.ID, Label: row.Title, Width: scale(28), Height: scale(32), Icon: woxcomponent.DisclosureChevronGlyph(scale(16), theme.Controls.BodyText, row.Expanded), OnTap: func() { c.toggleSidebarGroup(row.ID) }}), woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "notes.destination." + row.ID, Label: strings.Repeat("  ", row.Depth) + row.Title, Width: inner - scale(28), Height: scale(32), AlignLeading: true, Theme: theme.Controls, OnTap: func() {
				if c.organizeNotes(common.NotesTreeChange{Action: "move", ID: target.ID, Group: target.Group, ParentID: row.ID}) {
					c.closeSidebarDialog()
				}
			}})}})
		}

	}
	if s.errorText != "" {
		rows = append(rows, woxwidget.TextBlock{Value: s.errorText, Width: inner, Height: scale(48), LineHeight: scale(16), MaxLines: 3, Style: woxui.TextStyle{Size: scale(12)}, Color: theme.Controls.Error})
	}
	if s.dialog == "menu" {
		return c.sidebarActionMenu(size, theme, width, rows)
	}
	action("cancel", c.sidebarLabel("sidebar_cancel"), c.closeSidebarDialog)
	height := min(size.Height-scale(24), scale(32)+float32(len(rows))*scale(36))
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{ID: "notes.organize.dialog", Label: c.sidebarLabel("sidebar_root"), Width: width, Height: height, OverlayWidth: size.Width, OverlayHeight: size.Height, Theme: theme.Controls, Padding: woxwidget.UniformInsets(16), OnEscape: c.closeSidebarDialog, OnBackdrop: c.closeSidebarDialog, Child: woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{Key: "notes.organize.scroll", Width: inner, Height: height - 32, Theme: theme.Controls, Content: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: scale(4), Children: rows}})})
}

// sidebarDeleteDialog keeps the deletion workflow in the controller and dialog layout in the view.
func (c *notesWindowController) sidebarDeleteDialog(size woxui.Size, theme woxcomponent.Theme) woxwidget.Widget {
	target := c.sidebar.target
	message := c.sidebarLabel("note_delete_message")
	if target.Group {
		message = c.sidebarLabel("group_delete_message")
	}
	return launcherview.NotesDeleteDialog(launcherview.NotesDeleteDialogProps{
		Size: size, Theme: theme.Controls, Title: c.sidebarLabel("sidebar_delete"), Name: target.Title,
		Message: message, Error: c.sidebar.errorText, CancelLabel: c.sidebarLabel("sidebar_cancel"), DeleteLabel: c.sidebarLabel("sidebar_delete"),
		OnCancel: c.closeSidebarDialog, OnDelete: func() {
			if target.Group {
				if c.organizeNotes(common.NotesTreeChange{Action: "delete", ID: target.ID, Group: true}) {
					c.closeSidebarDialog()
				}
				return
			}
			if target.ID == c.record.ID {
				c.closeSidebarDialog()
				c.delete()
				return
			}
			if _, err := c.owner.services.NotesDelete(context.Background(), target.ID); err != nil {
				c.sidebar.errorText = err.Error()
				c.invalidate()
				return
			}
			c.closeSidebarDialog()
			c.refresh(target.ID)
		},
	})
}

// sidebarActionMenu anchors a compact menu to the row and keeps dismissal outside the menu undimmed.
func (c *notesWindowController) sidebarActionMenu(size woxui.Size, theme woxcomponent.Theme, width float32, rows []woxwidget.Widget) woxwidget.Widget {
	scale := theme.Controls.Scaled
	height := scale(16) + float32(len(rows)-1)*scale(32) + scale(1)
	left := min(max(scale(4), c.sidebar.menuAnchor.X), max(scale(4), size.Width-width-scale(4)))
	top := c.sidebar.menuAnchor.Y + c.sidebar.menuAnchor.Height
	if top+height > size.Height-scale(4) {
		top = c.sidebar.menuAnchor.Y - height
	}
	top = min(max(scale(4), top), max(scale(4), size.Height-height-scale(4)))
	background := theme.Controls.Surface
	if background.A == 0 {
		background = theme.Controls.InputBackground
		background.A = 255
	}
	menu := woxwidget.FocusScope{Key: "notes.organize.menu", Modal: true, Child: woxwidget.Semantics{AutomationID: "notes.organize.menu", Role: woxui.AccessibilityRoleMenu, Label: c.sidebarLabel("more"), Child: woxwidget.Container{Width: width, Height: height, Radius: scale(8), Floating: true, Color: background, BorderColor: theme.Controls.Border, BorderWidth: 1, Padding: woxwidget.UniformInsets(scale(8)), Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}}}
	return woxwidget.Stack{Width: size.Width, Height: size.Height, Children: []woxwidget.StackChild{
		{Child: woxwidget.Gesture{ID: "notes.organize.dismiss", OnTap: c.closeSidebarDialog, Child: woxwidget.Container{Width: size.Width, Height: size.Height}}},
		{Left: left, Top: top, Child: menu},
	}}
}

// sidebarGroupDialog follows the Chat project editor's title, field and action layout.
func (c *notesWindowController) sidebarGroupDialog(size woxui.Size, theme woxcomponent.Theme) woxwidget.Widget {
	s := &c.sidebar
	scale := theme.Controls.Scaled
	width := min(scale(460), max(0, size.Width-scale(24)))
	inner := max(0, width-scale(32))
	title := c.sidebarLabel("group_new")
	if s.dialog == "rename" {
		title = c.sidebarLabel("group_rename")
	} else if s.target.ID != "" {
		title = c.sidebarLabel("subgroup_new")
	}
	save := func() {
		if strings.TrimSpace(s.name.Text()) == "" {
			s.errorText = c.sidebarLabel("group_name_required")
			c.invalidate()
			return
		}
		change := common.NotesTreeChange{Action: s.dialog, ID: s.target.ID, Group: true, Name: s.name.Text()}
		if s.dialog == "create" {
			change.ParentID = s.target.ID
		}
		if c.organizeNotes(change) {
			c.closeSidebarDialog()
		}
	}
	children := []woxwidget.Widget{
		woxwidget.TextBlock{Value: title, Width: inner, Height: scale(32), LineHeight: scale(32), MaxLines: 1, AlignmentY: .5, Style: woxui.TextStyle{Size: scale(woxcomponent.SettingsPageTitleFontSize), Weight: woxui.FontWeightSemibold}, Color: theme.Controls.Text},
		woxcomponent.WoxTextField(woxcomponent.TextFieldProps{ID: "notes.group.name", Label: c.sidebarLabel("group_name"), Hint: c.sidebarLabel("group_name"), Width: inner, Height: scale(32), BorderColor: theme.Controls.Border, BorderWidth: 1, Controller: s.name, Value: s.name.Text(), Window: c.managed.Window(), Theme: theme.Controls, OnKey: func(event woxui.KeyEvent) bool {
			if event.Down && !event.Composing && event.Key == woxui.KeyEnter {
				save()
				return true
			}
			return false
		}}),
	}
	height := scale(152)
	if s.errorText != "" {
		children = append(children, woxwidget.TextBlock{Value: s.errorText, Width: inner, Height: scale(32), LineHeight: scale(16), MaxLines: 2, Style: woxui.TextStyle{Size: scale(12)}, Color: theme.Controls.Error})
		height += scale(44)
	}
	children = append(children, woxwidget.Align{Width: inner, Height: scale(32), Horizontal: 1, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: scale(8), CrossAxisAlignment: woxwidget.CrossAxisEnd, Children: []woxwidget.Widget{
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "notes.organize.cancel", Label: c.sidebarLabel("sidebar_cancel"), Height: scale(32), Theme: theme.Controls, OnTap: c.closeSidebarDialog}),
		woxcomponent.WoxButton(woxcomponent.ButtonProps{ID: "notes.organize.save", Label: c.sidebarLabel("sidebar_save"), Height: scale(32), Theme: theme.Controls, Variant: woxcomponent.ButtonPrimary, OnTap: save}),
	}}})
	return woxcomponent.WoxDialog(woxcomponent.DialogProps{ID: "notes.organize.dialog", Label: title, Width: width, Height: min(height, max(0, size.Height-scale(24))), OverlayWidth: size.Width, OverlayHeight: size.Height, Theme: theme.Controls, Padding: woxwidget.UniformInsets(scale(16)), BackdropAlpha: 160, InitialFocus: "notes.group.name", OnEscape: c.closeSidebarDialog, OnBackdrop: c.closeSidebarDialog, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: scale(12), Children: children}})
}

// noteContentWidth excludes a docked sidebar from editor measurement and image sizing.
func (c *notesWindowController) noteContentWidth() float32 {
	width := c.lastFrame.Width
	if width <= 0 {
		width = notesDefaultWidth
	}
	if c.sidebar.open {
		width -= c.controlTheme().Scaled(256)
	}
	return width
}

// persistSidebarScroll saves device-only offsets at window lifecycle boundaries, not on every pointer move.
func (c *notesWindowController) persistSidebarScroll() {
	if c.owner == nil || c.owner.services == nil || c.sidebar.scroll == nil {
		return
	}
	for _, entry := range []struct {
		key   string
		value float32
	}{{"sidebar.scroll", c.sidebar.scroll.Offset()}, {"sidebar.horizontal", c.sidebar.horizontal.Offset()}} {
		if err := c.owner.services.NotesSetLocal(context.Background(), entry.key, strconv.FormatFloat(float64(entry.value), 'f', -1, 32)); err != nil {
			c.fail(err)
		}
	}
}
