package launcher

import (
	"wox/common"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
)

type noteBlockRange = woxcomponent.NoteBlockRange

func projectNoteDocument(document common.NoteDocument, base woxui.TextStyle, theme woxcomponent.Theme) (string, []woxcomponent.NoteTextRun, []noteBlockRange) {
	return woxcomponent.ProjectNoteDocument(document, base, theme)
}

func documentFromEditor(value string, previous common.NoteDocument) common.NoteDocument {
	return woxcomponent.DocumentFromEditor(value, previous)
}

func toggleNoteInline(document common.NoteDocument, ranges []noteBlockRange, selection woxui.TextSelection, kind string, link string) common.NoteDocument {
	return woxcomponent.ToggleNoteInline(document, ranges, selection, kind, link)
}

func noteActiveFormats(document common.NoteDocument, ranges []noteBlockRange, selection woxui.TextSelection) map[string]bool {
	return woxcomponent.NoteActiveFormats(document, ranges, selection)
}

func noteActiveFormatsForTable(document common.NoteDocument, block, row, column int) map[string]bool {
	return woxcomponent.NoteActiveFormatsForTable(document, block, row, column)
}

func noteBlockAt(ranges []noteBlockRange, offset int) int {
	return woxcomponent.NoteBlockAt(ranges, offset)
}

func noteOpenableLink(target string) string {
	return woxcomponent.NoteOpenableLink(target)
}

func noteLinkAtOffset(document common.NoteDocument, ranges []noteBlockRange, offset int) string {
	return woxcomponent.NoteLinkAtOffset(document, ranges, offset)
}

func noteTaskAtOffset(document common.NoteDocument, ranges []noteBlockRange, offset int) (int, bool) {
	return woxcomponent.NoteTaskAtOffset(document, ranges, offset)
}

func noteDividerAtOffset(document common.NoteDocument, ranges []noteBlockRange, offset int) bool {
	return woxcomponent.NoteDividerAtOffset(document, ranges, offset)
}

func continueNoteBlock(document common.NoteDocument, ranges []noteBlockRange, selection woxui.TextSelection) (common.NoteDocument, int, bool) {
	return woxcomponent.ContinueNoteBlock(document, ranges, selection)
}

func adjustNoteListIndent(document common.NoteDocument, ranges []noteBlockRange, selection woxui.TextSelection, delta int) (common.NoteDocument, int, bool, bool) {
	return woxcomponent.AdjustNoteListIndent(document, ranges, selection, delta)
}

func cloneNoteDocument(document common.NoteDocument) common.NoteDocument {
	return woxcomponent.CloneNoteDocument(document)
}
