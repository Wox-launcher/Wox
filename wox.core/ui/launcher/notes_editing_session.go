package launcher

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"wox/common"
	notesplugin "wox/plugin/system/notes"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// noteUndoEntry restores the document, caret, and whole-note selection that belonged to it.
type noteUndoEntry struct {
	document         common.NoteDocument
	selection        woxui.TextSelection
	segment          woxcomponent.NoteDocumentSegment
	tableBlock       int
	imageBlock       int
	documentSelected bool
}

// notesEditingSession owns document changes, selection, projection, and undo history.
// It has no window, service, or timer dependency; effects are supplied by its host.
type notesEditingSession struct {
	editor            *woxwidget.TextEditingController
	editorFocus       *woxwidget.FocusNode
	requestTextFocus  bool
	record            common.NoteRecord
	document          common.NoteDocument
	richRuns          []woxcomponent.NoteTextRun
	blockRanges       []noteBlockRange
	selection         woxui.TextSelection
	focusedTableBlock int
	focusedTableRow   int
	focusedTableCol   int
	focusedImageBlock int
	documentSelected  bool
	markdownView      bool
	revealEditorCaret bool
	activeTextSegment woxcomponent.NoteDocumentSegment
	dirty             bool
	errorText         string
	zoom              float32
	undoDocuments     []noteUndoEntry
	redoDocuments     []noteUndoEntry
	lastTextEdit      time.Time
	taskReorderFrom   int
	taskReorderTo     int
	taskHover         int
	controlTheme      func() woxcomponent.ControlTheme
	onChanged         func()
	onInvalidate      func()
	revealCaret       func()
	mapVisualCaret    func(string, int, []woxcomponent.TextFieldRichRun, string, []woxcomponent.TextFieldRichRun) (int, bool)
}

// newNotesEditingSession can be used without a native window or persistence service.
func newNotesEditingSession() *notesEditingSession {
	return &notesEditingSession{
		editor: woxwidget.NewTextEditingController(""), editorFocus: woxwidget.NewFocusNode(),
		zoom: 1, focusedTableBlock: -1, focusedImageBlock: -1,
		taskReorderFrom: -1, taskReorderTo: -1, taskHover: -1,
		controlTheme: func() woxcomponent.ControlTheme { return woxcomponent.ControlTheme{} },
	}
}

// edited is the single notification point for document mutations and save retries.
func (c *notesEditingSession) edited() {
	c.dirty, c.errorText = true, ""
	if c.onChanged != nil {
		c.onChanged()
	}
	c.invalidate()
}

func (c *notesEditingSession) invalidate() {
	if c.onInvalidate != nil {
		c.onInvalidate()
	}
}

// visualCaret lets a window refine semantic caret mapping using its text metrics.
func (c *notesEditingSession) visualCaret(source string, offset int, sourceRuns []woxcomponent.TextFieldRichRun, dest string, destRuns []woxcomponent.TextFieldRichRun) (int, bool) {
	if c.mapVisualCaret == nil {
		return 0, false
	}
	return c.mapVisualCaret(source, offset, sourceRuns, dest, destRuns)
}

// load resets document-local selection and history when switching records.
func (c *notesEditingSession) load(record common.NoteRecord) {
	c.record, c.document = record, notesplugin.EnsureNoteImageEditGaps(record.Document)
	notesplugin.HydrateNoteImageDimensions(c.document)
	c.activeTextSegment = woxcomponent.NoteDocumentSegment{}
	c.focusedTableBlock = -1
	c.focusedImageBlock = -1
	c.documentSelected = false
	c.taskReorderFrom, c.taskReorderTo, c.taskHover = -1, -1, -1
	if c.markdownView {
		c.editor.SetText(notesplugin.ToMarkdown(c.document), false)
		c.richRuns, c.blockRanges = nil, nil
	} else {
		value, runs, ranges := c.projectActiveText()
		c.richRuns, c.blockRanges = runs, ranges
		c.editor.SetText(value, false)
	}
	c.selection = c.editor.State().Selection
	c.dirty, c.errorText = false, ""
	c.undoDocuments, c.redoDocuments = nil, nil
	c.lastTextEdit = time.Time{}
}

func (c *notesEditingSession) editorStyle() woxui.TextStyle {
	return woxui.TextStyle{Size: 14 * c.zoom, Family: woxui.FontFamilyUI}
}

// onEditorChanged converts the plain backing value into rich blocks and schedules persistence.
func (c *notesEditingSession) onEditorChanged(value string) {
	c.onSegmentChanged(c.activeTextSegment.Start, value)
}

func (c *notesEditingSession) onSegmentChanged(segmentStart int, value string) {
	previous := c.document
	c.rememberDocumentUndo(previous, time.Since(c.lastTextEdit) < 750*time.Millisecond)
	c.lastTextEdit = time.Now()
	segment := woxcomponent.NoteSegmentAtBlock(previous, segmentStart)
	if c.documentSelected {
		// The active field only holds one run; replace the whole note so typing after
		// select-all does not leave images and later paragraphs behind.
		c.documentSelected = false
		c.document = notesplugin.EnsureNoteImageEditGaps(documentFromEditor(value, common.NoteDocument{}))
	} else {
		parsed := documentFromEditor(value, woxcomponent.NoteSegmentDocument(previous, segment))
		c.document = woxcomponent.ReplaceNoteSegment(previous, segment, parsed.Blocks)
	}
	projected, runs, ranges := c.projectActiveText()
	c.richRuns, c.blockRanges = runs, ranges
	// SetText always moves the caret to the end. Keep the live editor selection
	// when the projection already matches, and restore it after a rewrite.
	if segment.Start == c.activeTextSegment.Start && projected != value {
		state := c.editor.State()
		delta := utf8.RuneCountInString(value) - utf8.RuneCountInString(projected)
		c.editor.SetText(projected, false)
		c.editor.SetSelection(max(0, state.Selection.Anchor-delta), max(0, state.Selection.Focus-delta))
	}
	c.selection = c.editor.State().Selection
	c.edited()
}

func (c *notesEditingSession) projectActiveText() (string, []woxcomponent.NoteTextRun, []noteBlockRange) {
	// Loaded, pasted, or edited documents can end in a table without a field for typing below it.
	if count := len(c.document.Blocks); count > 0 && c.document.Blocks[count-1].IsStructural() {
		c.document.Blocks = append(c.document.Blocks, common.NoteBlock{ID: newID(), Type: common.NoteBlockParagraph})
	}
	c.activeTextSegment = c.resolveActiveTextSegment()
	return woxcomponent.ProjectNoteSegment(c.document, c.activeTextSegment, c.editorStyle(), c.controlTheme())
}

// resolveActiveTextSegment keeps the caret in the text run the user last entered.
func (c *notesEditingSession) resolveActiveTextSegment() woxcomponent.NoteDocumentSegment {
	if c.activeTextSegment.End > c.activeTextSegment.Start && !c.activeTextSegment.Structural() {
		resolved := woxcomponent.NoteSegmentAtBlock(c.document, c.activeTextSegment.Start)
		if !resolved.Structural() {
			return resolved
		}
	}
	return c.preferredTextSegment()
}

func (c *notesEditingSession) firstTextSegment() woxcomponent.NoteDocumentSegment {
	for _, segment := range woxcomponent.NoteDocumentSegments(c.document) {
		if !segment.Structural() {
			return segment
		}
	}
	return woxcomponent.NoteDocumentSegment{Start: 0, End: len(c.document.Blocks)}
}

func (c *notesEditingSession) lastTextSegment() woxcomponent.NoteDocumentSegment {
	found := c.firstTextSegment()
	for _, segment := range woxcomponent.NoteDocumentSegments(c.document) {
		if !segment.Structural() {
			found = segment
		}
	}
	return found
}

// preferredTextSegment puts the caret below a leading image so the large empty editor area is editable.
func (c *notesEditingSession) preferredTextSegment() woxcomponent.NoteDocumentSegment {
	first := c.firstTextSegment()
	if first.End < len(c.document.Blocks) && c.document.Blocks[first.End].Type == common.NoteBlockImage && woxcomponent.NoteSegmentIsEmpty(c.document, first) {
		return c.lastTextSegment()
	}
	return first
}

// toggleMarkdownView switches the note between the rich preview and raw Markdown source.
func (c *notesEditingSession) toggleMarkdownView() {
	selection := c.editor.State().Selection
	source := c.editor.Text()
	sourceRuns := woxcomponent.NoteFieldRuns(c.richRuns)
	if c.markdownView {
		c.applyMarkdownToDocument()
		c.markdownView = false
		mapped := notesplugin.MapMarkdown(c.document)
		anchor := alignMarkdownOffset(source, mapped.Markdown, selection.Anchor)
		focus := alignMarkdownOffset(source, mapped.Markdown, selection.Focus)
		block, imageBlock, tableBlock, textOff := noteMarkdownCaretTarget(c.document, mapped, focus)
		anchorBlock, _, _, textOffAnchor := noteMarkdownCaretTarget(c.document, mapped, anchor)
		c.focusedImageBlock = imageBlock
		c.focusedTableBlock = tableBlock
		c.activeTextSegment = noteSegmentForCaretBlock(c.document, block)
		value, runs, ranges := c.projectActiveText()
		c.richRuns, c.blockRanges = runs, ranges
		c.editor.SetText(value, false)
		next := woxui.TextSelection{
			Anchor: notePreviewOffsetForBlock(c.blockRanges, anchorBlock, textOffAnchor),
			Focus:  notePreviewOffsetForBlock(c.blockRanges, block, textOff),
		}
		if visual, ok := c.visualCaret(source, selection.Focus, sourceRuns, value, woxcomponent.NoteFieldRuns(runs)); ok {
			next = woxui.TextSelection{Anchor: visual, Focus: visual}
		}
		c.setEditorSelection(next, utf8.RuneCountInString(value))
	} else {
		mapped := notesplugin.MapMarkdown(c.document)
		next := notePreviewSelectionToMarkdown(c.document, c.blockRanges, selection, c.focusedImageBlock, c.focusedTableBlock, mapped)
		c.markdownView = true
		c.focusedTableBlock = -1
		c.focusedImageBlock = -1
		c.editor.SetText(mapped.Markdown, false)
		if visual, ok := c.visualCaret(source, selection.Focus, sourceRuns, mapped.Markdown, nil); ok {
			next = woxui.TextSelection{Anchor: visual, Focus: visual}
		}
		c.setEditorSelection(next, utf8.RuneCountInString(mapped.Markdown))
	}
	if c.documentSelected {
		c.editor.SelectAll()
		c.selection = c.editor.State().Selection
		if c.markdownView {
			c.documentSelected = false
		}
	}
	c.revealEditorCaret = true
	if c.editorFocus != nil {
		c.editorFocus.RequestFocus()
	}
	if c.revealCaret != nil {
		c.revealCaret()
	}
	c.invalidate()
}

// applyMarkdownToDocument parses the raw source editor into the durable document.
func (c *notesEditingSession) applyMarkdownToDocument() {
	if !c.markdownView {
		return
	}
	c.document = notesplugin.NormalizeDocument(notesplugin.ParseMarkdown(c.editor.Text()))
	notesplugin.HydrateNoteImageDimensions(c.document)
}

// onMarkdownChanged keeps the durable document in sync while the source editor is open.
func (c *notesEditingSession) onMarkdownChanged(value string) {
	c.rememberDocumentUndo(c.document, time.Since(c.lastTextEdit) < 750*time.Millisecond)
	c.lastTextEdit = time.Now()
	c.document = notesplugin.NormalizeDocument(notesplugin.ParseMarkdown(value))
	c.edited()
}

// focusImage selects one attachment so its action bar can appear.
func (c *notesEditingSession) focusImage(block int) {
	c.documentSelected = false
	c.focusedImageBlock = block
	c.focusedTableBlock = -1
	c.invalidate()
}

// focusTextBesideImage puts the caret in the paragraph before or after the selected image.
func (c *notesEditingSession) focusTextBesideImage(block int, after bool) {
	id := ""
	if block >= 0 && block < len(c.document.Blocks) {
		id = c.document.Blocks[block].ID
	}
	c.document = notesplugin.EnsureNoteImageEditGaps(c.document)
	if id != "" {
		for index, item := range c.document.Blocks {
			if item.ID == id {
				block = index
				break
			}
		}
	}
	target := block
	if after {
		target = min(block+1, len(c.document.Blocks)-1)
	} else if block > 0 {
		target = block - 1
	} else {
		target = 0
	}
	c.bindActiveText(target, !after)
	// The shared FocusNode still belongs to the previous segment until the next build.
	// Let the target field request focus after it has attached that node.
	c.requestTextFocus = true
}

// focusNoteText moves editing into a text run after the user taps it.
func (c *notesEditingSession) focusNoteText(segmentStart int) {
	if c.focusedImageBlock < 0 && c.focusedTableBlock < 0 && c.activeTextSegment.Start == segmentStart && (c.editorFocus == nil || c.editorFocus.HasFocus()) {
		return
	}
	c.bindActiveText(segmentStart, true)
}

// bindActiveText projects a text run without requesting focus through the previous segment's attachment.
func (c *notesEditingSession) bindActiveText(segmentStart int, caretAtEnd bool) {
	c.documentSelected = false
	c.focusedImageBlock = -1
	c.focusedTableBlock = -1
	c.activeTextSegment = woxcomponent.NoteSegmentAtBlock(c.document, segmentStart)
	if c.activeTextSegment.Structural() {
		c.activeTextSegment = c.preferredTextSegment()
	}
	value, runs, ranges := woxcomponent.ProjectNoteSegment(c.document, c.activeTextSegment, c.editorStyle(), c.controlTheme())
	c.richRuns, c.blockRanges = runs, ranges
	c.editor.SetText(value, false)
	if caretAtEnd {
		end := utf8.RuneCountInString(value)
		c.editor.SetSelection(end, end)
	} else {
		c.editor.SetSelection(0, 0)
	}
	c.selection = c.editor.State().Selection
	c.invalidate()
}

// scaleImage changes display width while preserving the image's aspect ratio.
func (c *notesEditingSession) scaleImage(block, delta int) {
	if block < 0 || block >= len(c.document.Blocks) || c.document.Blocks[block].Image == nil {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	updated := woxcomponent.CloneNoteDocument(c.document)
	updated.Blocks[block].Image.Scale = notesplugin.AdjustNoteImageScale(updated.Blocks[block].Image.Scale, delta)
	c.document = updated
	c.focusedImageBlock = block
	c.edited()
}

// deleteImage removes the focused attachment block from the document.
func (c *notesEditingSession) deleteImage(block int) {
	if block < 0 || block >= len(c.document.Blocks) || c.document.Blocks[block].Type != common.NoteBlockImage {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = woxcomponent.DeleteNoteImage(c.document, block)
	c.focusedImageBlock = -1
	c.reproject(false)
}

// onEditorSelectionChanged stores the live caret and drops a whole-note selection when it shrinks.
func (c *notesEditingSession) onEditorSelectionChanged(selection woxui.TextSelection) {
	if selection == c.selection {
		return
	}
	c.selection = selection
	c.focusedTableBlock = -1
	c.focusedImageBlock = -1
	if c.documentSelected && !c.editorHasFullSelection() {
		c.documentSelected = false
	}
	c.invalidate()
}

// selectEntireDocument selects every block so Ctrl+A is not stopped by an image or table.
func (c *notesEditingSession) selectEntireDocument() bool {
	if c.markdownView || !noteDocumentHasStructuralBlocks(c.document) {
		return false
	}
	c.documentSelected = true
	c.focusedImageBlock = -1
	c.focusedTableBlock = -1
	if c.editorFocus != nil {
		c.editorFocus.RequestFocus()
	}
	c.editor.SelectAll()
	c.selection = c.editor.State().Selection
	c.invalidate()
	return true
}

// clearDocumentSelection deletes a whole-note selection, including images and tables.
func (c *notesEditingSession) clearDocumentSelection() bool {
	if !c.documentSelected || c.record.DeletedAt > 0 {
		return false
	}
	return c.replaceSelectedDocument(notesplugin.EmptyDocument())
}

// replaceSelectedDocument swaps the current note for pasted or cleared content.
func (c *notesEditingSession) replaceSelectedDocument(next common.NoteDocument) bool {
	c.rememberDocumentUndo(c.document, false)
	c.documentSelected = false
	c.focusedImageBlock = -1
	c.focusedTableBlock = -1
	c.document = notesplugin.EnsureNoteImageEditGaps(notesplugin.NormalizeDocument(next))
	notesplugin.HydrateNoteImageDimensions(c.document)
	c.activeTextSegment = woxcomponent.NoteDocumentSegment{}
	if c.markdownView {
		c.editor.SetText(notesplugin.ToMarkdown(c.document), false)
		c.selection = c.editor.State().Selection
		c.edited()
		return true
	}
	c.reproject(true)
	return true
}

func (c *notesEditingSession) editorHasFullSelection() bool {
	state := c.editor.State()
	limit := utf8.RuneCountInString(state.Text)
	if limit == 0 {
		return true
	}
	return !state.Selection.Collapsed() && state.Selection.Start() == 0 && state.Selection.End() == limit
}

func noteDocumentHasStructuralBlocks(document common.NoteDocument) bool {
	for _, block := range document.Blocks {
		if block.IsStructural() {
			return true
		}
	}
	return false
}

// continueBlock applies Enter directly to the block model before the plain text field inserts a newline.
func (c *notesEditingSession) continueBlock() bool {
	if c.record.DeletedAt > 0 {
		return false
	}
	document, block, handled := continueNoteBlock(c.document, c.blockRanges, c.editor.State().Selection)
	if !handled {
		return false
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = document
	c.reproject(false)
	// Document block numbers are not slice indexes after an image or table splits the editor.
	c.editor.SetCaret(woxcomponent.NoteRangeForBlock(c.blockRanges, block).TextStart)
	c.selection = c.editor.State().Selection
	c.invalidate()
	return true
}

func (c *notesEditingSession) changeListIndent(delta int) bool {
	if c.record.DeletedAt > 0 {
		return false
	}
	selection := c.editor.State().Selection
	document, block, changed, handled := adjustNoteListIndent(c.document, c.blockRanges, selection, delta)
	if !handled || !changed {
		return handled
	}
	oldTextStart := woxcomponent.NoteRangeForBlock(c.blockRanges, block).TextStart
	c.rememberDocumentUndo(c.document, false)
	c.document = document
	c.reproject(false)
	shift := woxcomponent.NoteRangeForBlock(c.blockRanges, block).TextStart - oldTextStart
	c.editor.SetCaret(selection.Focus + shift)
	c.selection = c.editor.State().Selection
	c.invalidate()
	return true
}

func (c *notesEditingSession) toggleInline(kind string) {
	if c.record.DeletedAt > 0 {
		return
	}
	if c.focusedTableBlock >= 0 {
		c.rememberDocumentUndo(c.document, false)
		c.document = woxcomponent.ToggleNoteTableInline(c.document, c.focusedTableBlock, c.focusedTableRow, c.focusedTableCol, kind, "")
		c.reproject(false)
		return
	}
	if c.selection.Collapsed() {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = toggleNoteInline(c.document, c.blockRanges, c.selection, kind, "")
	c.reproject(false)
}

func (c *notesEditingSession) setBlock(blockType common.NoteBlockType) {
	if c.record.DeletedAt > 0 || len(c.document.Blocks) == 0 || c.focusedTableBlock >= 0 {
		return
	}
	index := noteBlockAt(c.blockRanges, c.selection.Focus)
	if index >= 0 && index < len(c.document.Blocks) && c.document.Blocks[index].Type != common.NoteBlockTable {
		c.rememberDocumentUndo(c.document, false)
		c.document.Blocks[index].Type = blockType
		if blockType != common.NoteBlockTask {
			c.document.Blocks[index].Checked = false
		}
		if blockType != common.NoteBlockBullet && blockType != common.NoteBlockOrdered && blockType != common.NoteBlockTask {
			c.document.Blocks[index].Indent = 0
		}
		c.document.Blocks[index].Table = nil
		c.reproject(false)
	}
}

func (c *notesEditingSession) insertTable() {
	if c.record.DeletedAt > 0 {
		return
	}
	c.rememberDocumentUndo(c.document, false)
	document, index := woxcomponent.InsertNoteTable(c.document, c.blockRanges, c.selection)
	c.document = document
	c.focusedTableBlock, c.focusedTableRow, c.focusedTableCol = index, 0, 0
	c.reproject(false)
}

func (c *notesEditingSession) cycleBlock() {
	if len(c.document.Blocks) == 0 || c.focusedTableBlock >= 0 {
		return
	}
	index := noteBlockAt(c.blockRanges, c.selection.Focus)
	if index < 0 || index >= len(c.document.Blocks) {
		return
	}
	sequence := []common.NoteBlockType{common.NoteBlockParagraph, common.NoteBlockHeading1, common.NoteBlockHeading2, common.NoteBlockHeading3, common.NoteBlockCode}
	current := slices.Index(sequence, c.document.Blocks[index].Type)
	c.setBlock(sequence[(current+1)%len(sequence)])
}

func (c *notesEditingSession) toggleTask() {
	if len(c.document.Blocks) == 0 {
		return
	}
	c.toggleTaskBlock(noteBlockAt(c.blockRanges, c.selection.Focus))
}

// appendParagraphBelowText lets blank-space clicks leave a formatted block without changing Enter's continuation rules.
func (c *notesEditingSession) appendParagraphBelowText() bool {
	if c.record.DeletedAt > 0 || len(c.document.Blocks) == 0 {
		return false
	}
	if last := c.document.Blocks[len(c.document.Blocks)-1]; last.Type == common.NoteBlockParagraph && last.Text == "" {
		return false
	}
	c.rememberDocumentUndo(c.document, false)
	c.document.Blocks = append(c.document.Blocks, common.NoteBlock{ID: newID(), Type: common.NoteBlockParagraph})
	c.reproject(false)
	c.editor.SetCaret(utf8.RuneCountInString(c.editor.Text()))
	c.selection = c.editor.State().Selection
	return true
}

// editorCursorAt distinguishes links and task markers from ordinary editable text.
func (c *notesEditingSession) editorCursorAt(offset int) woxui.PointerCursor {
	if _, ok := noteTaskAtOffset(c.document, c.blockRanges, offset); ok {
		return woxui.PointerCursorHand
	}
	if noteLinkAtOffset(c.document, c.blockRanges, offset) != "" {
		return woxui.PointerCursorHand
	}
	if noteDividerAtOffset(c.document, c.blockRanges, offset) {
		return woxui.PointerCursorDefault
	}
	return woxui.PointerCursorText
}

// hoverTask reveals the left-gutter handle for the checklist item under the pointer.
func (c *notesEditingSession) hoverTask(block int) {
	if c.taskHover == block {
		return
	}
	c.taskHover = block
	c.invalidate()
}

// startTaskReorder begins a checklist drag without rewriting the document until drop.
func (c *notesEditingSession) startTaskReorder(block int) {
	if c.record.DeletedAt > 0 || block < 0 || block >= len(c.document.Blocks) || c.document.Blocks[block].Type != common.NoteBlockTask {
		return
	}
	c.taskReorderFrom, c.taskReorderTo = block, block
}

// endTaskReorder commits the pending checklist move.
func (c *notesEditingSession) endTaskReorder() {
	from, dest := c.taskReorderFrom, c.taskReorderTo
	c.taskReorderFrom, c.taskReorderTo = -1, -1
	if from < 0 {
		return
	}
	c.applyTaskMove(from, dest)
}

// nudgeTaskBlock moves the caret's checklist item one slot with Alt+Arrow.
func (c *notesEditingSession) nudgeTaskBlock(delta int) bool {
	index := woxcomponent.NoteTaskAtCaret(c.document, c.blockRanges, c.selection)
	if index < 0 || c.record.DeletedAt > 0 {
		return false
	}
	dest := woxcomponent.NoteTaskNudgeDest(c.document, index, delta)
	return c.applyTaskMove(index, dest)
}

// applyTaskMove writes one checklist reorder through the shared undo and save path.
func (c *notesEditingSession) applyTaskMove(from, dest int) bool {
	updated, next := woxcomponent.MoveNoteTaskGroup(c.document, from, dest)
	if next == from && noteDocumentsEqual(updated, c.document) {
		c.invalidate()
		return false
	}
	c.rememberDocumentUndo(c.document, false)
	c.document = updated
	c.reproject(false)
	c.focusNoteBlock(next)
	return true
}

// focusNoteBlock puts the caret on the moved task so the handle stays on that item.
func (c *notesEditingSession) focusNoteBlock(index int) {
	for _, blockRange := range c.blockRanges {
		if blockRange.Block == index {
			c.editor.SetCaret(blockRange.TextStart)
			c.selection = c.editor.State().Selection
			return
		}
	}
}

func noteDocumentsEqual(left, right common.NoteDocument) bool {
	if len(left.Blocks) != len(right.Blocks) {
		return false
	}
	for index := range left.Blocks {
		if left.Blocks[index].ID != right.Blocks[index].ID {
			return false
		}
	}
	return true
}

// toggleTaskBlock updates one task through the same undo and autosave path as keyboard formatting.
func (c *notesEditingSession) toggleTaskBlock(index int) bool {
	if c.record.DeletedAt > 0 || index < 0 || index >= len(c.document.Blocks) || c.document.Blocks[index].Type != common.NoteBlockTask {
		return false
	}
	c.rememberDocumentUndo(c.document, false)
	c.document.Blocks[index].Checked = !c.document.Blocks[index].Checked
	c.reproject(false)
	return true
}

// reproject updates visible rich runs after a document-level formatting change.
func (c *notesEditingSession) reproject(resetSelection bool) {
	state := c.editor.State()
	value, runs, ranges := c.projectActiveText()
	c.richRuns, c.blockRanges = runs, ranges
	if value != state.Text {
		c.editor.SetText(value, false)
		if !resetSelection {
			c.editor.SetSelection(min(state.Selection.Anchor, utf8.RuneCountInString(value)), min(state.Selection.Focus, utf8.RuneCountInString(value)))
		}
	}
	c.selection = c.editor.State().Selection
	c.edited()
}

func (c *notesEditingSession) pasteDocument(value string) bool {
	pasted := notesplugin.ParseClipboard(value)
	if !noteClipboardHasStructure(pasted) {
		return false
	}
	return c.insertPastedDocument(pasted)
}

// insertPastedDocument places structured clipboard blocks at the caret or table focus.
func (c *notesEditingSession) insertPastedDocument(pasted common.NoteDocument) bool {
	if len(pasted.Blocks) == 0 {
		return false
	}
	if c.documentSelected {
		return c.replaceSelectedDocument(pasted)
	}
	c.rememberDocumentUndo(c.document, false)
	index := 0
	if c.focusedTableBlock >= 0 {
		index = c.focusedTableBlock
	} else if len(c.blockRanges) > 0 {
		index = noteBlockAt(c.blockRanges, c.selection.Focus)
	}
	insertAt := index + 1
	if index < 0 || index >= len(c.document.Blocks) {
		insertAt = len(c.document.Blocks)
		c.document.Blocks = append(c.document.Blocks, pasted.Blocks...)
	} else if c.document.Blocks[index].Type == common.NoteBlockParagraph && strings.TrimSpace(c.document.Blocks[index].Text) == "" {
		insertAt = index
		c.document.Blocks = slices.Replace(c.document.Blocks, index, index+1, pasted.Blocks...)
	} else {
		c.document.Blocks = slices.Insert(c.document.Blocks, index+1, pasted.Blocks...)
	}
	c.focusedImageBlock = -1
	if pasted.Blocks[0].Type == common.NoteBlockTable {
		c.focusedTableBlock, c.focusedTableRow, c.focusedTableCol = index+1, 0, 0
		if index < len(c.document.Blocks) && c.document.Blocks[index].Type == common.NoteBlockTable && strings.TrimSpace(c.document.Blocks[index].Text) == "" {
			c.focusedTableBlock = index
		}
	} else {
		for offset, block := range pasted.Blocks {
			if block.Type != common.NoteBlockImage {
				continue
			}
			c.focusedTableBlock = -1
			c.focusedImageBlock = insertAt + offset
			break
		}
	}
	if c.markdownView {
		c.editor.SetText(notesplugin.ToMarkdown(c.document), false)
		c.selection = c.editor.State().Selection
		c.edited()
		return true
	}
	c.reproject(false)
	return true
}

func noteClipboardHasStructure(document common.NoteDocument) bool {
	if len(document.Blocks) != 1 {
		return true
	}
	return document.Blocks[0].Type == common.NoteBlockTable || document.Blocks[0].Type == common.NoteBlockImage
}
