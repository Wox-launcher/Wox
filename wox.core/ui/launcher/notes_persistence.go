package launcher

import (
	"context"
	"fmt"
	"time"

	notesplugin "wox/plugin/system/notes"
	"wox/util"
)

// notesAutosave debounces writes; generation rejects callbacks already queued before
// a flush, record switch, close, or shutdown. Its mutable state stays on the UI thread.
type notesAutosave struct {
	saveTimer      *time.Timer
	saveGeneration uint64
	dispatch       func(string, func()) error
	stopped        func() bool
	save           func() error
	invalidate     func()
}

const notesAutosaveDelay = 500 * time.Millisecond

// scheduleSave coalesces typing into the 500 ms autosave boundary.
func (c *notesAutosave) scheduleSave() {
	c.cancelScheduledSave()
	if c.stopped() {
		return
	}
	generation := c.saveGeneration
	c.saveTimer = time.AfterFunc(notesAutosaveDelay, func() {
		_ = c.dispatch("autosave note", func() {
			if c.stopped() || c.saveGeneration != generation {
				return
			}
			if err := c.save(); err != nil {
				util.GetLogger().Error(context.Background(), fmt.Sprintf("autosave note: %v", err))
			}
			c.invalidate()
		})
	})
}

// cancelScheduledSave also invalidates timer callbacks already queued on the UI thread.
func (c *notesAutosave) cancelScheduledSave() {
	c.saveGeneration++
	if c.saveTimer != nil {
		c.saveTimer.Stop()
		c.saveTimer = nil
	}
}

// flush persists dirty state while keeping failed edits retryable.
func (c *notesWindowController) flush() error {
	c.cancelScheduledSave()
	c.applyMarkdownToDocument()
	if !c.dirty || c.record.ID == "" || c.owner == nil || c.owner.services == nil {
		return nil
	}
	if notesplugin.DocumentIsEmpty(c.document) {
		return nil
	}
	c.saving = true
	previousID := c.record.ID
	result, err := c.owner.services.NotesSave(context.Background(), c.record.ID, c.record.Revision, c.document)
	c.saving = false
	if err != nil {
		c.errorText = c.owner.language.translate("i18n:notes_save_failed") + ": " + err.Error()
		return err
	}
	c.record = result.Record
	c.document = result.Record.Document
	c.dirty = false
	if c.record.ID != previousID {
		if c.owner.noteWindows[previousID] == c {
			delete(c.owner.noteWindows, previousID)
		}
		c.owner.noteWindows[c.record.ID] = c
	}
	if result.Conflict {
		c.errorText = c.owner.language.translate("i18n:notes_conflict_copy")
	} else {
		c.errorText = ""
	}
	return c.reloadSummaries()
}

// discardEmptyNote drops an untitled draft so opening a note without typing does not persist it.
func (c *notesWindowController) discardEmptyNote() {
	if c == nil || c.record.ID == "" || !notesplugin.DocumentIsEmpty(c.document) {
		return
	}
	c.cancelScheduledSave()
	c.dirty = false
	if err := c.owner.services.NotesDiscard(context.Background(), c.record.ID); err != nil {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("discard empty note: %v", err))
	}
}

// persistCurrentNote saves edits, or drops an unused empty draft, before this window changes notes.
func (c *notesWindowController) persistCurrentNote() error {
	c.applyMarkdownToDocument()
	if notesplugin.DocumentIsEmpty(c.document) {
		c.discardEmptyNote()
		return nil
	}
	return c.flush()
}
