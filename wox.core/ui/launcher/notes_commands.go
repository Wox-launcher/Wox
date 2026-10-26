package launcher

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"wox/common"
	notesplugin "wox/plugin/system/notes"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	"wox/util"
	"wox/util/clipboard"
)

// reloadSummaries refreshes search, pin, and trash rows for the current filter.
func (c *notesWindowController) reloadSummaries() error {
	query := ""
	if c.searchOpen {
		query = strings.TrimSpace(c.searchEditor.Text())
	}
	items, err := c.owner.services.NotesList(context.Background(), query, true)
	if err != nil {
		c.errorText = err.Error()
		return err
	}
	c.summaries = items
	c.clampSearchIndex()
	return nil
}

// refresh applies external sync changes only when no dirty local edit would be replaced.
func (c *notesWindowController) refresh(noteID string) {
	_ = c.reloadSummaries()
	if c.record.ID == "" || (noteID != "" && noteID != c.record.ID) || c.dirty {
		c.invalidate()
		return
	}
	if record, err := c.owner.services.NotesGet(context.Background(), c.record.ID); err == nil && record.Revision != c.record.Revision {
		c.applyRecord(record)
	} else if err != nil && !notesplugin.DocumentIsEmpty(c.document) {
		c.errorText = err.Error()
	}
	c.invalidate()
}

// searchItems returns notes in overlay order: pinned, recent, then recently deleted.
func (c *notesWindowController) searchItems() []common.NoteSummary {
	items := make([]common.NoteSummary, 0, len(c.summaries))
	appendMatching := func(match func(common.NoteSummary) bool) {
		for _, item := range c.summaries {
			if match(item) {
				items = append(items, item)
			}
		}
	}
	appendMatching(func(item common.NoteSummary) bool { return item.PinnedAt > 0 && item.DeletedAt == 0 })
	appendMatching(func(item common.NoteSummary) bool { return item.PinnedAt == 0 && item.DeletedAt == 0 })
	appendMatching(func(item common.NoteSummary) bool { return item.DeletedAt > 0 })
	return items
}

// clampSearchIndex keeps keyboard selection on a visible row after the filtered list changes.
func (c *notesWindowController) clampSearchIndex() {
	count := len(c.searchItems())
	if count == 0 {
		c.searchIndex = 0
		return
	}
	c.searchIndex = min(max(0, c.searchIndex), count-1)
}

func (c *notesWindowController) moveSearchSelection(delta int) {
	c.searchIndex += delta
	c.clampSearchIndex()
	c.invalidate()
}

// openSearchItem shows the chosen note in this window instead of opening another utility window.
func (c *notesWindowController) openSearchItem(summary common.NoteSummary) {
	if summary.ID == c.record.ID && summary.DeletedAt == 0 {
		c.searchOpen = false
		c.editorFocus.RequestFocus()
		c.invalidate()
		return
	}
	if err := c.persistCurrentNote(); err != nil {
		c.fail(err)
		return
	}
	var record common.NoteRecord
	var err error
	if summary.DeletedAt > 0 {
		record, err = c.owner.services.NotesRestore(context.Background(), summary.ID)
	} else {
		record, err = c.owner.services.NotesGet(context.Background(), summary.ID)
	}
	if err != nil {
		c.fail(err)
		return
	}
	if err := c.showNoteInCurrentWindow(record); err != nil {
		c.fail(err)
		return
	}
	c.searchOpen = false
	c.editorFocus.RequestFocus()
	c.invalidate()
}

// showNoteInCurrentWindow rebinds this native window to another note.
func (c *notesWindowController) showNoteInCurrentWindow(record common.NoteRecord) error {
	if record.ID == "" {
		return fmt.Errorf("note ID is required")
	}
	if record.ID == c.record.ID {
		return nil
	}
	if existing := c.owner.noteWindows[record.ID]; existing != nil && existing != c && existing.managed != nil && existing.managed.Lifecycle() != woxui.WindowLifecycleClosed {
		c.searchOpen = false
		c.invalidate()
		return existing.open(common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: record.ID})
	}
	previousID := c.record.ID
	c.persistBounds()
	if c.owner.noteWindows[previousID] == c {
		delete(c.owner.noteWindows, previousID)
	}
	if c.owner.noteWindows == nil {
		c.owner.noteWindows = map[string]*notesWindowController{}
	}
	c.applyRecord(record)
	c.owner.noteWindows[record.ID] = c
	c.owner.activateNoteWindow(c)
	c.searchEditor.SetText("", false)
	c.searchIndex = 0
	return c.reloadSummaries()
}

func (c *notesWindowController) activateSearchSelection() {
	items := c.searchItems()
	if len(items) == 0 {
		return
	}
	c.openSearchItem(items[c.searchIndex])
}

func (c *notesWindowController) runAction(action func() error) {
	if err := action(); err != nil {
		c.fail(err)
	}
	c.invalidate()
}

func (c *notesWindowController) fail(err error) {
	if err != nil {
		c.errorText = err.Error()
		util.GetLogger().Error(context.Background(), fmt.Sprintf("Notes action: %v", err))
	}
	c.invalidate()
}

// toggleSearch switches focus between the editor and keyboard-first note browser.
func (c *notesWindowController) toggleSearch() {
	c.searchOpen, c.moreOpen, c.linkOpen = !c.searchOpen, false, false
	if c.searchOpen {
		c.searchIndex = 0
		_ = c.reloadSummaries()
		c.searchFocus.RequestFocus()
	} else {
		c.editorFocus.RequestFocus()
	}
	c.invalidate()
}

func (c *notesWindowController) onSearchKey(event woxui.KeyEvent) bool {
	if !event.Down || event.Composing {
		return false
	}
	switch event.Key {
	case woxui.KeyEscape:
		c.searchOpen = false
		c.editorFocus.RequestFocus()
		c.invalidate()
		return true
	case woxui.KeyArrowDown:
		c.moveSearchSelection(1)
		return true
	case woxui.KeyArrowUp:
		c.moveSearchSelection(-1)
		return true
	case woxui.KeyEnter:
		c.activateSearchSelection()
		return true
	default:
		return false
	}
}

func (c *notesWindowController) onLinkKey(event woxui.KeyEvent) bool {
	if !event.Down || event.Composing {
		return false
	}
	switch event.Key {
	case woxui.KeyEscape:
		c.linkOpen = false
		c.editorFocus.RequestFocus()
		c.invalidate()
		return true
	case woxui.KeyEnter:
		c.applyLink()
		return true
	default:
		return false
	}
}

func (c *notesWindowController) onKey(event woxui.KeyEvent) bool {
	if !event.Down || event.Composing {
		return false
	}
	if event.Key == woxui.KeyEscape {
		if c.searchOpen || c.moreOpen || c.linkOpen {
			c.searchOpen, c.moreOpen, c.linkOpen = false, false, false
			c.editorFocus.RequestFocus()
			c.invalidate()
			return true
		}
		if c.focusedImageBlock >= 0 {
			c.focusTextBesideImage(c.focusedImageBlock, true)
			return true
		}
		c.requestClose()
		return true
	}
	if c.documentSelected && (event.Key == woxui.KeyBackspace || event.Key == woxui.KeyDelete) && event.Modifiers == 0 {
		return c.clearDocumentSelection()
	}
	if c.focusedImageBlock >= 0 && event.Modifiers == 0 {
		switch event.Key {
		case woxui.KeyBackspace, woxui.KeyDelete:
			c.deleteImage(c.focusedImageBlock)
			return true
		case woxui.KeyArrowLeft, woxui.KeyArrowUp:
			c.focusTextBesideImage(c.focusedImageBlock, false)
			return true
		case woxui.KeyArrowRight, woxui.KeyArrowDown, woxui.KeyEnter:
			c.focusTextBesideImage(c.focusedImageBlock, true)
			return true
		}
	}
	if event.Key == woxui.KeyEnter && event.Modifiers == 0 && c.continueBlock() {
		return true
	}
	if event.Key == woxui.KeyTab && event.Modifiers&^woxui.KeyModifierShift == 0 {
		delta := 1
		if event.Modifiers&woxui.KeyModifierShift != 0 {
			delta = -1
		}
		return c.changeListIndent(delta)
	}
	if event.Modifiers&woxui.KeyModifierAlt != 0 && event.Modifiers&^woxui.KeyModifierAlt == 0 {
		switch event.Key {
		case woxui.KeyArrowUp:
			return c.nudgeTaskBlock(-1)
		case woxui.KeyArrowDown:
			return c.nudgeTaskBlock(1)
		}
	}
	if !event.Modifiers.HasPrimary() {
		return false
	}
	switch event.Key {
	case woxui.Key("a"):
		return c.selectEntireDocument()
	case woxui.Key("c"):
		return c.copyDocumentSelection()
	case woxui.Key("x"):
		if event.Modifiers&woxui.KeyModifierShift != 0 {
			c.toggleInline("strike")
			return true
		}
		return c.cutDocumentSelection()
	case woxui.Key("p"):
		if event.Modifiers&woxui.KeyModifierShift != 0 {
			c.toggleWindowPin()
		} else {
			c.toggleSearch()
		}
	case woxui.Key("n"):
		c.runAction(c.owner.openNewNoteWindow)
	case woxui.Key("b"):
		c.toggleInline("bold")
	case woxui.Key("i"):
		c.toggleInline("italic")
	case woxui.Key("u"):
		c.toggleInline("underline")
	case woxui.Key("e"):
		c.toggleMarkdownView()
	case woxui.Key("k"):
		c.openLink()
	case woxui.Key("v"):
		return c.pasteFromClipboard("")
	case woxui.Key("z"):
		if event.Modifiers&woxui.KeyModifierShift != 0 {
			return c.redoDocument()
		}
		return c.undoDocument()
	case woxui.Key("y"):
		return c.redoDocument()
	case woxui.KeyEnter:
		c.toggleTask()
	case woxui.Key("0"):
		c.setZoom(1)
	case woxui.Key("+"), woxui.Key("="):
		c.setZoom(c.zoom + .1)
	case woxui.Key("-"):
		c.setZoom(c.zoom - .1)
	default:
		if event.Key >= woxui.Key("1") && event.Key <= woxui.Key("9") {
			index := int(event.Key[0] - '1')
			pinned := make([]common.NoteSummary, 0)
			for _, item := range c.summaries {
				if item.PinnedAt > 0 && item.DeletedAt == 0 {
					pinned = append(pinned, item)
				}
			}
			if index < len(pinned) {
				c.runAction(func() error {
					return c.owner.openNoteRequest(common.NotesWindowRequest{Action: common.NotesWindowOpen, NoteID: pinned[index].ID})
				})
			}
			return true
		}
		return false
	}
	return true
}

// copyDocumentSelection writes the full note Markdown when select-all crossed structural blocks.
func (c *notesWindowController) copyDocumentSelection() bool {
	if !c.documentSelected {
		return false
	}
	if err := clipboard.WriteText(notesplugin.ToMarkdown(c.document)); err != nil {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("note copy: %v", err))
	}
	return true
}

// cutDocumentSelection copies the full note and replaces it with an empty document.
func (c *notesWindowController) cutDocumentSelection() bool {
	if !c.documentSelected {
		return false
	}
	c.copyDocumentSelection()
	return c.clearDocumentSelection()
}

func (c *notesWindowController) openLink() {
	if c.record.DeletedAt > 0 || (c.selection.Collapsed() && c.focusedTableBlock < 0) {
		return
	}
	c.linkEditor.SetText("https://", true)
	c.linkOpen, c.moreOpen, c.searchOpen = true, false, false
	c.linkFocus.RequestFocus()
	c.invalidate()
}

func (c *notesWindowController) applyLink() {
	link := strings.TrimSpace(c.linkEditor.Text())
	if link == "" {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	if c.focusedTableBlock >= 0 {
		c.document = woxcomponent.ToggleNoteTableInline(c.document, c.focusedTableBlock, c.focusedTableRow, c.focusedTableCol, "link", link)
	} else {
		c.document = toggleNoteInline(c.document, c.blockRanges, c.selection, "link", link)
	}
	c.linkOpen = false
	c.reproject(false)
	c.editorFocus.RequestFocus()
}

func (c *notesWindowController) handleBlockTap(offset int) bool {
	if index, ok := noteTaskAtOffset(c.document, c.blockRanges, offset); ok {
		return c.toggleTaskBlock(index)
	}
	if noteDividerAtOffset(c.document, c.blockRanges, offset) {
		return true
	}
	if target := noteLinkAtOffset(c.document, c.blockRanges, offset); target != "" {
		c.openNoteLink(target)
		return true
	}
	return false
}

// openNoteLink opens a persisted note URL in the desktop browser.
func (c *notesWindowController) openNoteLink(target string) {
	if c.managed == nil {
		return
	}
	if err := c.managed.Window().OpenExternalURL(target); err != nil {
		c.fail(err)
	}
}

// dragTaskReorder records the insert-before index under the pointer so the editor can preview it.
func (c *notesWindowController) dragTaskReorder(block int, contentY float32) {
	if c.taskReorderFrom < 0 {
		c.startTaskReorder(block)
	}
	if c.taskReorderFrom < 0 {
		return
	}
	dest := c.taskReorderDest(contentY)
	if dest == c.taskReorderTo {
		return
	}
	c.taskReorderTo = dest
	c.invalidate()
}

// taskReorderDest maps the pointer's document Y onto an insert-before index.
func (c *notesWindowController) taskReorderDest(contentY float32) int {
	lineHeight := float32(24) * c.zoom
	if lineHeight <= 0 {
		lineHeight = 24
	}
	innerWidth := c.lastFrame.Width - 32
	if innerWidth <= 0 {
		innerWidth = 400
	}
	var window *woxui.Window
	if c.managed != nil {
		window = c.managed.Window()
	}
	return woxcomponent.NoteTaskLiveDest(c.document, c.blockRanges, c.taskReorderFrom, contentY, lineHeight, innerWidth, c.editor.Text(), c.richRuns, c.editorStyle(), window)
}

// pasteFromClipboard inserts files, a clipboard bitmap, or structured text at the caret.
func (c *notesWindowController) pasteFromClipboard(value string) bool {
	if c.record.DeletedAt > 0 {
		return true
	}
	if strings.TrimSpace(value) == "" {
		if text, err := clipboard.ReadText(); err == nil {
			value = text
		}
	}
	img, imgErr := clipboard.ReadImage()
	if imgErr != nil {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("note paste: read image: %v", imgErr))
	}
	if paths, err := clipboard.ReadFilePaths(); err == nil && len(paths) > 0 {
		if pasted := notesplugin.DocumentFromClipboardFiles(paths); noteDocumentHasImage(pasted) || img == nil {
			if len(pasted.Blocks) > 0 {
				return c.insertPastedDocument(pasted)
			}
		}
	}
	if img != nil && !noteClipboardTextOutranksImage(value, img) {
		return c.pasteImportedImage(img, "clipboard.png")
	}
	if strings.TrimSpace(value) != "" {
		return c.pasteDocument(value)
	}
	return false
}

// noteClipboardTextOutranksImage keeps Word/browser prose instead of their DIB preview.
// A lone URL or filename next to a real bitmap is the usual "copy image" side-channel.
func noteClipboardTextOutranksImage(value string, img image.Image) bool {
	value = strings.TrimSpace(value)
	if value == "" || img == nil {
		return false
	}
	if strings.ContainsAny(value, " \t\n\r") {
		return true
	}
	bounds := img.Bounds()
	return bounds.Dx() < 64 || bounds.Dy() < 64
}

func noteDocumentHasImage(document common.NoteDocument) bool {
	for _, block := range document.Blocks {
		if block.Type == common.NoteBlockImage {
			return true
		}
	}
	return false
}

// pasteClipboardFiles imports copied image files and turns other paths into links.
func (c *notesWindowController) pasteClipboardFiles(paths []string) bool {
	pasted := notesplugin.DocumentFromClipboardFiles(paths)
	if len(pasted.Blocks) == 0 {
		return false
	}
	return c.insertPastedDocument(pasted)
}

// pasteImportedImage stores a clipboard bitmap as a note attachment and inserts it.
func (c *notesWindowController) pasteImportedImage(img image.Image, fileName string) bool {
	pasted, err := notesplugin.DocumentFromClipboardImage(img, fileName)
	if err != nil {
		c.errorText = c.owner.language.translate("i18n:notes_paste_failed")
		util.GetLogger().Error(context.Background(), fmt.Sprintf("paste note image: %v", err))
		c.invalidate()
		return true
	}
	return c.insertPastedDocument(pasted)
}

func (c *notesWindowController) pasteTableClipboard(block, row, column int, value string) bool {
	return c.pasteFromClipboard(value)
}

func (c *notesWindowController) setZoom(value float32) {
	c.zoom = max(float32(.75), min(float32(2), value))
	if !c.markdownView {
		projected, runs, ranges := c.projectActiveText()
		c.richRuns, c.blockRanges = runs, ranges
		if projected != c.editor.Text() {
			selection := c.editor.State().Selection
			c.editor.SetText(projected, false)
			c.editor.SetSelection(selection.Anchor, selection.Focus)
		}
	}
	_ = c.owner.services.NotesSetLocal(context.Background(), c.preferenceKey("zoom"), fmt.Sprintf("%.2f", c.zoom))
	c.invalidate()
}

func (c *notesWindowController) copyLink() {
	if c.record.ID == "" {
		return
	}
	link := "wox://plugin/" + common.NotesPluginID + "?action=open&id=" + c.record.ID
	if err := clipboard.WriteText(link); err != nil {
		c.fail(err)
	}
}

// delete soft-deletes the note and closes only its bound window.
func (c *notesWindowController) delete() {
	if c.record.ID == "" {
		return
	}
	c.runAction(func() error {
		if notesplugin.DocumentIsEmpty(c.document) {
			c.discardEmptyNote()
			return c.close()
		}
		if err := c.flush(); err != nil {
			return err
		}
		_, err := c.owner.services.NotesDelete(context.Background(), c.record.ID)
		if err != nil {
			return err
		}
		return c.close()
	})
}

func (c *notesWindowController) restore() {
	if c.record.ID == "" {
		return
	}
	c.runAction(func() error {
		record, err := c.owner.services.NotesRestore(context.Background(), c.record.ID)
		if err == nil {
			c.applyRecord(record)
			err = c.reloadSummaries()
		}
		return err
	})
}

// export flushes first, then writes the selected codec through the native save dialog.
func (c *notesWindowController) export(format string) {
	c.runAction(func() error {
		if err := c.flush(); err != nil {
			return err
		}
		exported, err := c.owner.services.NotesExport(context.Background(), c.record.ID, format)
		if err != nil {
			return err
		}
		title := "Untitled Note"
		if custom := notesplugin.CustomNoteTitle(c.document); custom != "" {
			title = custom
		}
		name := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`).ReplaceAllString(title, "-")
		name = strings.Trim(strings.TrimSpace(name), ".")
		if name == "" {
			name = "Untitled Note"
		}
		path, err := c.managed.Window().SaveFile(woxui.SaveFileOptions{Title: c.owner.language.translate("i18n:notes_export"), DefaultFileName: name + "." + exported.Extension, Extension: exported.Extension})
		if err != nil || path == "" {
			return err
		}
		if filepath.Ext(path) == "" {
			path += "." + exported.Extension
		}
		return os.WriteFile(path, []byte(exported.Content), 0o600)
	})
}

// resolveNoteImage loads a note attachment or remote picture at preview resolution
// and pins it while the block stays in the editor viewport.
func (c *notesWindowController) resolveNoteImage(image common.NoteImage) *woxui.Image {
	source, size, ok := c.noteImageCacheSource(image)
	if !ok {
		return nil
	}
	return c.owner.imageForViewport(source, size)
}

// releaseNoteImage drops the viewport pin after the block scrolls away.
func (c *notesWindowController) releaseNoteImage(image common.NoteImage) {
	source, size, ok := c.noteImageCacheSource(image)
	if !ok {
		return
	}
	c.owner.releaseViewportImage(source, size)
}

func (c *notesWindowController) noteImageCacheSource(image common.NoteImage) (woxImage, int, bool) {
	if c.owner == nil {
		return woxImage{}, 0, false
	}
	width := c.lastFrame.Width
	if width <= 0 {
		width = notesDefaultWidth
	}
	height := woxcomponent.NoteEditorImageMaxHeight * max(c.zoom, 1)
	size := previewImageRequestSize(width, height)
	if remote := notesplugin.NoteImageRemoteURL(image); remote != "" {
		return woxImage{ImageType: "url", ImageData: remote}, size, true
	}
	path := notesplugin.ResolveNoteImagePath(image)
	if path == "" {
		return woxImage{}, 0, false
	}
	return woxImage{ImageType: "absolute", ImageData: path}, size, true
}
