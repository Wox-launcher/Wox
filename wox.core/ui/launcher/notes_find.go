package launcher

import (
	"fmt"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// findMatches follows the visible document order without touching the editing selection.
func (c *notesWindowController) findMatches() []woxcomponent.NoteFindTarget {
	if !c.findOpen || c.findEditor == nil || c.findEditor.Text() == "" {
		return nil
	}
	var matches []woxcomponent.NoteFindTarget
	add := func(text string, block, row, column int) {
		for _, match := range woxcomponent.FindTextMatches(text, c.findEditor.Text()) {
			matches = append(matches, woxcomponent.NoteFindTarget{Block: block, Row: row, Column: column, Offset: match.Start()})
		}
	}
	if c.markdownView {
		add(c.editor.Text(), -1, -1, -1)
		return matches
	}
	for _, segment := range woxcomponent.NoteDocumentSegments(c.document) {
		if segment.Table {
			table := c.document.Blocks[segment.Start].Table
			if table != nil {
				for row, cells := range table.Rows {
					for col, cell := range cells {
						add(cell.Text, segment.Start, row, col)
					}
				}
			}
		} else if !segment.Image {
			text, _, _ := woxcomponent.ProjectNoteSegment(c.document, segment, c.editorStyle(), c.controlTheme())
			add(text, segment.Start, -1, -1)
		}
	}
	return matches
}

// currentFind derives a valid target after edits or switching the note representation.
func (c *notesWindowController) currentFind() (string, *woxcomponent.NoteFindTarget) {
	if !c.findOpen || c.findEditor == nil {
		return "", nil
	}
	matches := c.findMatches()
	if len(matches) == 0 {
		return c.findEditor.Text(), nil
	}
	return c.findEditor.Text(), &matches[min(c.findIndex, len(matches)-1)]
}

func (c *notesWindowController) moveFind(delta int) {
	c.findReveal = true
	matches := c.findMatches()
	if len(matches) > 0 {
		c.findIndex = (min(c.findIndex, len(matches)-1) + delta + len(matches)) % len(matches)
	}
	c.invalidate()
}

func (c *notesWindowController) closeFind() {
	c.findOpen = false
	c.editorFocus.RequestFocus()
	c.invalidate()
}

// onFindKey runs before editor dispatch so tables and the search field share the shortcuts.
func (c *notesWindowController) onFindKey(event woxui.KeyEvent) bool {
	if !event.Down || event.Composing {
		return false
	}
	if event.Modifiers.HasPrimary() && event.Key == woxui.Key("f") {
		if !event.Repeat {
			c.findOpen = true
			c.findReveal = true
			c.searchOpen, c.moreOpen, c.linkOpen = false, false, false
			c.findFocus.RequestFocus()
			c.findEditor.SelectAll()
			c.invalidate()
		}
		return true
	}
	if !c.findOpen {
		return false
	}
	if event.Key == woxui.KeyEscape {
		c.closeFind()
		return true
	}
	if event.Key == woxui.KeyEnter && c.findFocus.HasFocus() {
		delta := 1
		if event.Modifiers&woxui.KeyModifierShift != 0 {
			delta = -1
		}
		c.moveFind(delta)
		return true
	}
	return false
}

// buildFindOverlay keeps the compact find bar above the editor without a modal scrim.
func (c *notesWindowController) buildFindOverlay(size woxui.Size, theme woxcomponent.Theme, overlay woxwidget.Widget) woxwidget.Widget {
	matches := c.findMatches()
	index := 0
	if len(matches) > 0 {
		index = min(c.findIndex, len(matches)-1) + 1
	}
	return launcherview.NotesFindBar(launcherview.NotesFindProps{
		Size: size, Theme: theme, Overlay: overlay, Window: c.managed.Window(),
		Editor: c.findEditor, Focus: c.findFocus, Count: fmt.Sprintf("%d/%d", index, len(matches)),
		Label: c.owner.language.translate("i18n:notes_find"), PreviousLabel: c.owner.language.translate("i18n:notes_find_previous"), NextLabel: c.owner.language.translate("i18n:notes_find_next"), CloseLabel: c.owner.language.translate("i18n:notes_find_close"),
		Enabled: len(matches) > 0, OnPrevious: func() { c.moveFind(-1) }, OnNext: func() { c.moveFind(1) }, OnClose: c.closeFind, OnKey: c.onFindKey,
		OnChanged: func(string) { c.findIndex = 0; c.findReveal = true; c.invalidate() },
	})
}

// findEditorPadding keeps the first result below the floating search bar.
func (c *notesWindowController) findEditorPadding() woxwidget.Insets {
	padding := notesEditorPadding()
	if c.findOpen {
		padding.Top += c.controlTheme().Scaled(44)
	}
	return padding
}
