package component

import (
	"testing"

	"wox/common"
	woxui "wox/ui/runtime"
)

func TestDocumentFromEditorKeepsRepeatedLinkLabel(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{{
		Type: common.NoteBlockParagraph,
		Text: "Wox经历过两届主要维护者。然后第二届主要维护者",
		Spans: []common.NoteSpan{
			{Start: 6, End: 13, Link: "https://example.com/a"},
			{Start: 16, End: 24, Link: "https://example.com/b"},
		},
	}}}
	parsed := DocumentFromEditor("Wox经历过两轮主要维护者。然后第二届主要维护者", document)
	if len(parsed.Blocks) != 1 || parsed.Blocks[0].Text != "Wox经历过两轮主要维护者。然后第二届主要维护者" {
		t.Fatalf("edited text = %#v", parsed.Blocks)
	}
	if len(parsed.Blocks[0].Spans) != 2 || parsed.Blocks[0].Spans[0].Link != "https://example.com/a" || parsed.Blocks[0].Spans[0].Start != 6 || parsed.Blocks[0].Spans[0].End != 13 {
		t.Fatalf("first link after edit = %#v", parsed.Blocks[0].Spans)
	}
	if parsed.Blocks[0].Spans[1].Link != "https://example.com/b" || parsed.Blocks[0].Spans[1].Start != 16 || parsed.Blocks[0].Spans[1].End != 24 {
		t.Fatalf("second link after edit = %#v", parsed.Blocks[0].Spans)
	}
}

func TestNoteTaskGroupIncludesIndentedChildren(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{ID: "a", Type: common.NoteBlockTask, Text: "parent"},
		{ID: "b", Type: common.NoteBlockTask, Text: "child", Indent: 1},
		{ID: "c", Type: common.NoteBlockTask, Text: "other"},
	}}
	start, end := NoteTaskGroup(document, 0)
	if start != 0 || end != 2 {
		t.Fatalf("parent group = [%d, %d), want [0, 2)", start, end)
	}
	start, end = NoteTaskGroup(document, 1)
	if start != 1 || end != 2 {
		t.Fatalf("child group = [%d, %d), want [1, 2)", start, end)
	}
}

func TestMoveNoteTaskGroupReordersWithChildren(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{ID: "a", Type: common.NoteBlockTask, Text: "first"},
		{ID: "b", Type: common.NoteBlockTask, Text: "second"},
		{ID: "c", Type: common.NoteBlockTask, Text: "child", Indent: 1},
		{ID: "d", Type: common.NoteBlockParagraph, Text: "tail"},
	}}
	moved, index := MoveNoteTaskGroup(document, 1, 0)
	if index != 0 || moved.Blocks[0].ID != "b" || moved.Blocks[1].ID != "c" || moved.Blocks[2].ID != "a" || moved.Blocks[3].ID != "d" {
		t.Fatalf("move second to top = index %d blocks %#v", index, moved.Blocks)
	}
	moved, index = MoveNoteTaskGroup(document, 1, 4)
	if index != 2 || moved.Blocks[0].ID != "a" || moved.Blocks[1].ID != "d" || moved.Blocks[2].ID != "b" || moved.Blocks[3].ID != "c" {
		t.Fatalf("move second past tail = index %d blocks %#v", index, moved.Blocks)
	}
	unchanged, index := MoveNoteTaskGroup(document, 1, 1)
	if index != 1 || unchanged.Blocks[1].ID != "b" {
		t.Fatalf("no-op move = index %d blocks %#v", index, unchanged.Blocks)
	}
}

func TestNoteTaskLiveDestUsesLineMidpoints(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{ID: "a", Type: common.NoteBlockTask, Text: "one"},
		{ID: "b", Type: common.NoteBlockTask, Text: "two"},
		{ID: "c", Type: common.NoteBlockTask, Text: "three"},
	}}
	_, _, ranges := ProjectNoteDocument(document, woxui.TextStyle{Size: 14}, Theme{})
	lines := []textFieldLine{
		{start: ranges[0].Start, end: ranges[0].End},
		{start: ranges[1].Start, end: ranges[1].End},
		{start: ranges[2].Start, end: ranges[2].End},
	}
	if dest := noteTaskLiveDest(document, ranges, 1, 0, 24, lines); dest != 0 {
		t.Fatalf("above first midpoint = %d, want 0", dest)
	}
	if dest := noteTaskLiveDest(document, ranges, 1, 36, 24, lines); dest != 1 {
		t.Fatalf("over self = %d, want 1", dest)
	}
	if dest := noteTaskLiveDest(document, ranges, 1, 70, 24, lines); dest != 3 {
		t.Fatalf("past last midpoint = %d, want 3", dest)
	}
}

func TestNoteTaskDropAndNudgeDest(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{ID: "a", Type: common.NoteBlockTask, Text: "one"},
		{ID: "b", Type: common.NoteBlockTask, Text: "two"},
		{ID: "c", Type: common.NoteBlockTask, Text: "three"},
	}}
	_, _, ranges := ProjectNoteDocument(document, woxui.TextStyle{Size: 14}, Theme{})
	if dest := NoteTaskDropDest(document, ranges, 1, ranges[0].TextStart); dest != 0 {
		t.Fatalf("drop onto first = %d, want 0", dest)
	}
	if dest := NoteTaskDropDest(document, ranges, 1, ranges[2].TextStart); dest != 3 {
		t.Fatalf("drop onto last = %d, want 3", dest)
	}
	if dest := NoteTaskDropDest(document, ranges, 1, ranges[1].TextStart); dest != 1 {
		t.Fatalf("drop onto self = %d, want 1", dest)
	}
	if dest := NoteTaskNudgeDest(document, 1, -1); dest != 0 {
		t.Fatalf("nudge up = %d, want 0", dest)
	}
	if dest := NoteTaskNudgeDest(document, 1, 1); dest != 3 {
		t.Fatalf("nudge down = %d, want 3", dest)
	}
}

func TestProjectNoteDocumentRendersEmptyParagraphsAsBlankLines(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{Type: common.NoteBlockParagraph, Text: "first"},
		{Type: common.NoteBlockParagraph},
		{Type: common.NoteBlockParagraph, Text: "second"},
	}}
	value, _, _ := ProjectNoteDocument(document, woxui.TextStyle{Size: 14}, Theme{})
	if value != "first\n\nsecond" {
		t.Fatalf("projection = %q, want a visible blank line between paragraphs", value)
	}
}

func TestNoteTaskAtCaret(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{ID: "p", Type: common.NoteBlockParagraph, Text: "plain"},
		{ID: "t", Type: common.NoteBlockTask, Text: "task"},
	}}
	_, _, ranges := ProjectNoteDocument(document, woxui.TextStyle{Size: 14}, Theme{})
	if index := NoteTaskAtCaret(document, ranges, woxui.TextSelection{Focus: ranges[0].TextStart}); index != -1 {
		t.Fatalf("caret in paragraph = %d, want -1", index)
	}
	if index := NoteTaskAtCaret(document, ranges, woxui.TextSelection{Focus: ranges[1].TextStart}); index != 1 {
		t.Fatalf("caret in task = %d, want 1", index)
	}
}

func TestNoteEditorTaskReorderHandleFollowsCaret(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{ID: "p", Type: common.NoteBlockParagraph, Text: "plain"},
		{ID: "t", Type: common.NoteBlockTask, Text: "task"},
	}}
	_, _, ranges := ProjectNoteDocument(document, woxui.TextStyle{Size: 14}, Theme{})
	segment := NoteDocumentSegment{Start: 0, End: 2}
	started := false
	props := NoteEditorProps{
		Document: document, Selection: woxui.TextSelection{Focus: ranges[1].TextStart}, Focused: true,
		ReorderingTask: -1, OnReorderTaskStart: func(int) { started = true }, OnReorderTaskDrag: func(int, float32) {},
	}
	handle := noteEditorTaskReorderHandle(props, segment, ranges)
	if handle == nil || handle.Start != ranges[1].Start || handle.End != ranges[1].End || handle.Size != 16 {
		t.Fatalf("caret on task handle = %#v, want range %+v", handle, ranges[1])
	}
	handle.OnPanStart()
	if !started {
		t.Fatal("handle pan must start a reorder")
	}
	props.Selection = woxui.TextSelection{Focus: ranges[0].TextStart}
	if handle := noteEditorTaskReorderHandle(props, segment, ranges); handle != nil {
		t.Fatal("caret on a paragraph must hide the reorder handle")
	}
	hovered := -2
	props.Focused = false
	props.HoveredTask = 1
	props.OnHoverTask = func(block int) { hovered = block }
	handle = noteEditorTaskReorderHandle(props, segment, ranges)
	if handle == nil || handle.Start != ranges[1].Start {
		t.Fatalf("hover on task handle = %#v", handle)
	}
	noteEditorHoverTask(props, ranges)(ranges[1].TextStart, true)
	if hovered != 1 {
		t.Fatalf("hover task = %d, want 1", hovered)
	}
}
