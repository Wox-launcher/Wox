package system

import (
	"testing"
	"wox/util"
)

func TestClipboardMetadataRegistersPasteCommand(t *testing.T) {
	commands := (&ClipboardPlugin{}).GetMetadata().Commands
	for _, command := range commands {
		if command.Command == clipboardPasteCommand {
			if command.Description != "i18n:plugin_clipboard_command_paste_description" {
				t.Fatalf("paste command description = %q", command.Description)
			}
			return
		}
	}
	t.Fatal("paste command missing from clipboard metadata")
}

func TestResolvePasteCursorEmptyMeansNewest(t *testing.T) {
	records := []ClipboardRecord{
		{ID: "c", Content: "C"},
		{ID: "b", Content: "B"},
		{ID: "a", Content: "A"},
	}

	record, cursorID, found := resolvePasteCursor(records, "")
	if !found {
		t.Fatal("expected newest record")
	}
	if record.ID != "c" || cursorID != "" {
		t.Fatalf("record=%s cursor=%q, want newest C with empty cursor", record.ID, cursorID)
	}
}

func TestAdvancePasteCursorWalksNewestToOldestAndStays(t *testing.T) {
	records := []ClipboardRecord{
		{ID: "c", Content: "C"},
		{ID: "b", Content: "B"},
		{ID: "a", Content: "A"},
	}

	cursor := advancePasteCursor(records, "")
	if cursor != "b" {
		t.Fatalf("after C: cursor=%q, want b", cursor)
	}
	cursor = advancePasteCursor(records, cursor)
	if cursor != "a" {
		t.Fatalf("after B: cursor=%q, want a", cursor)
	}
	cursor = advancePasteCursor(records, cursor)
	if cursor != "a" {
		t.Fatalf("after A: cursor=%q, want to stay on a", cursor)
	}
}

func TestResolvePasteCursorMissingIDFallsBackToNewest(t *testing.T) {
	records := []ClipboardRecord{
		{ID: "c", Content: "C"},
		{ID: "b", Content: "B"},
	}

	record, cursorID, found := resolvePasteCursor(records, "missing")
	if !found {
		t.Fatal("expected fallback record")
	}
	if record.ID != "c" || cursorID != "" {
		t.Fatalf("record=%s cursor=%q, want newest C with cleared cursor", record.ID, cursorID)
	}
}

func TestResolvePasteCursorEmptyHistory(t *testing.T) {
	if _, _, found := resolvePasteCursor(nil, "c"); found {
		t.Fatal("empty history must not resolve a record")
	}
	if cursor := advancePasteCursor(nil, "c"); cursor != "" {
		t.Fatalf("empty history advance = %q, want empty", cursor)
	}
}

func TestNextPasteRecordDescribesOlderItem(t *testing.T) {
	records := []ClipboardRecord{
		{ID: "c", Type: "text", Content: "C"},
		{ID: "b", Type: "text", Content: "B"},
		{ID: "a", Type: "text", Content: "A"},
	}

	next, found := nextPasteRecord(records, "")
	if !found || next.ID != "b" {
		t.Fatalf("next after newest = %+v found=%v, want B", next, found)
	}
	next, found = nextPasteRecord(records, "b")
	if !found || next.ID != "a" {
		t.Fatalf("next after B = %+v found=%v, want A", next, found)
	}
	if _, found = nextPasteRecord(records, "a"); found {
		t.Fatal("oldest item must not expose a next record")
	}
}

func TestClipboardRecordDescriptionPrefersAlias(t *testing.T) {
	alias := "receipt"
	record := ClipboardRecord{Type: "text", Content: "a very long clipboard body", Alias: &alias}
	if got := clipboardRecordDescription(record); got != alias {
		t.Fatalf("description = %q, want alias", got)
	}
}

func TestResetPasteCursorClearsID(t *testing.T) {
	plugin := &ClipboardPlugin{pasteCursorID: "b"}
	plugin.resetPasteCursor()
	if plugin.pasteCursorID != "" {
		t.Fatalf("pasteCursorID=%q, want empty after user copy reset", plugin.pasteCursorID)
	}
}

func TestShouldSkipPasteHistoryCapture(t *testing.T) {
	if !shouldSkipPasteHistoryCapture(2000, 1000) {
		t.Fatal("expected skip while window is open")
	}
	if shouldSkipPasteHistoryCapture(1000, 1000) {
		t.Fatal("skip must expire at the window boundary")
	}
	if shouldSkipPasteHistoryCapture(0, 1000) {
		t.Fatal("unset skip window must not block capture")
	}
}

func TestSkipHistoryCaptureDoesNotResetCursor(t *testing.T) {
	plugin := &ClipboardPlugin{
		pasteCursorID:    "b",
		skipHistoryUntil: util.GetSystemTimestamp() + pasteHistorySkipWindowMs,
	}
	if !plugin.shouldSkipHistoryCapture() {
		t.Fatal("expected active skip window")
	}
	if plugin.pasteCursorID != "b" {
		t.Fatal("skip-history window must not reset the paste cursor")
	}
}
