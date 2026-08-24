package launcher

import (
	"testing"

	"wox/common"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
)

func TestNotesEditorRoundTripAndMarkdownRules(t *testing.T) {
	document := common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
		{ID: "title", Type: common.NoteBlockHeading1, Text: "Roadmap", Spans: []common.NoteSpan{{Start: 0, End: 7, Bold: true}}},
		{ID: "task", Type: common.NoteBlockTask, Text: "Ship it"},
	}}
	value, runs, _ := projectNoteDocument(document, woxui.TextStyle{Size: 14}, woxcomponent.Theme{QueryBackground: woxui.Color{A: 20}})
	if value != "Roadmap\n☐ Ship it" || len(runs) == 0 {
		t.Fatalf("unexpected projection: %q %#v", value, runs)
	}
	parsed := documentFromEditor("Roadmap\n- [x] **Ship** `it`", document)
	if parsed.Blocks[1].Type != common.NoteBlockTask || !parsed.Blocks[1].Checked || parsed.Blocks[1].Text != "Ship it" {
		t.Fatalf("unexpected parsed task: %#v", parsed.Blocks[1])
	}
	if len(parsed.Blocks[1].Spans) != 2 || !parsed.Blocks[1].Spans[0].Bold || !parsed.Blocks[1].Spans[1].Code {
		t.Fatalf("inline rules were not retained: %#v", parsed.Blocks[1].Spans)
	}
}

func TestNotesDividerProjectsAsHorizontalRule(t *testing.T) {
	dividerColor := woxui.Color{R: 80, G: 90, B: 100, A: 255}
	document := common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "divider", Type: common.NoteBlockDivider}}}
	value, runs, ranges := projectNoteDocument(document, woxui.TextStyle{Size: 14}, woxcomponent.Theme{PreviewSplit: dividerColor})
	if value != "────────" || len(runs) != 1 || !runs[0].HorizontalRule || runs[0].Color != dividerColor {
		t.Fatalf("divider projection = %q %#v, want one semantic horizontal rule", value, runs)
	}
	if !noteDividerAtOffset(document, ranges, ranges[0].Start) || !noteDividerAtOffset(document, ranges, ranges[0].End) {
		t.Fatal("divider placeholder offsets must be treated as one atomic visual block")
	}
}

func TestToggleNoteInlineAcrossBlocks(t *testing.T) {
	document := common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
		{ID: "a", Type: common.NoteBlockParagraph, Text: "one"},
		{ID: "b", Type: common.NoteBlockParagraph, Text: "two"},
	}}
	_, _, ranges := projectNoteDocument(document, woxui.TextStyle{Size: 14}, woxcomponent.Theme{})
	document = toggleNoteInline(document, ranges, woxui.TextSelection{Anchor: 1, Focus: 6}, "italic", "")
	if len(document.Blocks[0].Spans) != 1 || len(document.Blocks[1].Spans) != 1 || !document.Blocks[0].Spans[0].Italic || !document.Blocks[1].Spans[0].Italic {
		t.Fatalf("cross-block selection was not formatted: %#v", document.Blocks)
	}
}

func TestDocumentFromEditorConvertsFencedCodePaste(t *testing.T) {
	document := documentFromEditor("```go\n# first()\nsecond()\n```", common.NoteDocument{})
	if len(document.Blocks) != 2 || document.Blocks[0].Type != common.NoteBlockCode || document.Blocks[1].Type != common.NoteBlockCode {
		t.Fatalf("fenced code paste was not converted: %#v", document.Blocks)
	}
	if document.Blocks[0].Text != "# first()" {
		t.Fatalf("code content was parsed as Markdown: %#v", document.Blocks[0])
	}
}

func TestContinueNoteBlockCreatesTheNextMarker(t *testing.T) {
	for _, blockType := range []common.NoteBlockType{common.NoteBlockBullet, common.NoteBlockOrdered, common.NoteBlockTask, common.NoteBlockQuote} {
		document := common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "item", Type: blockType, Text: "first"}}}
		_, runs, ranges := projectNoteDocument(document, woxui.TextStyle{Size: 14}, woxcomponent.Theme{})
		if blockType == common.NoteBlockQuote && (len(runs) == 0 || !runs[0].LeadingBar || runs[0].End != ranges[0].TextEnd) {
			t.Fatalf("quote did not project a continuous leading bar: %#v", runs)
		}
		selection := woxui.TextSelection{Anchor: ranges[0].TextEnd, Focus: ranges[0].TextEnd}
		updated, active, handled := continueNoteBlock(document, ranges, selection)
		if !handled || active != 1 || len(updated.Blocks) != 2 || updated.Blocks[1].Type != blockType || updated.Blocks[1].Text != "" || updated.Blocks[1].Checked {
			t.Fatalf("%s list was not continued: %#v", blockType, updated.Blocks)
		}
	}
}

func TestCheckedTaskUsesMutedTextAndClickablePrefix(t *testing.T) {
	muted := woxui.Color{R: 120, G: 120, B: 120, A: 255}
	accent := woxui.Color{R: 40, G: 130, B: 230, A: 255}
	document := common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "task", Type: common.NoteBlockTask, Text: "done", Checked: true}}}
	_, runs, ranges := projectNoteDocument(document, woxui.TextStyle{Size: 14}, woxcomponent.Theme{ResultSubtitle: muted, Cursor: accent})
	if len(runs) != 2 || !runs[0].Checkbox || !runs[0].Checked || runs[0].Color != accent || runs[1].Color != muted {
		t.Fatalf("checked task color = %#v, want %#v", runs, muted)
	}
	if index, ok := noteTaskAtOffset(document, ranges, ranges[0].Start); !ok || index != 0 {
		t.Fatal("task checkbox prefix was not clickable")
	}
	if _, ok := noteTaskAtOffset(document, ranges, ranges[0].TextStart+1); ok {
		t.Fatal("task text must keep normal caret behavior")
	}
}

func TestNotesListIndentSupportsThreeLevels(t *testing.T) {
	document := common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
		{ID: "parent", Type: common.NoteBlockTask, Text: "parent"},
		{ID: "child", Type: common.NoteBlockTask, Text: "child", Indent: 1},
		{ID: "grandchild", Type: common.NoteBlockTask, Text: "grandchild", Indent: 1},
	}}
	_, _, ranges := projectNoteDocument(document, woxui.TextStyle{Size: 14}, woxcomponent.Theme{})
	selection := woxui.TextSelection{Anchor: ranges[2].TextEnd, Focus: ranges[2].TextEnd}
	indented, block, changed, handled := adjustNoteListIndent(document, ranges, selection, 1)
	if !handled || !changed || block != 2 || indented.Blocks[2].Indent != 2 {
		t.Fatalf("third-level indent = %#v, block %d, changed %t, handled %t", indented.Blocks, block, changed, handled)
	}
	value, runs, indentedRanges := projectNoteDocument(indented, woxui.TextStyle{Size: 14}, woxcomponent.Theme{})
	thirdCheckbox := false
	for _, run := range runs {
		thirdCheckbox = thirdCheckbox || run.Checkbox && run.Start == indentedRanges[2].Marker
	}
	if value != "☐ parent\n    ☐ child\n        ☐ grandchild" || !thirdCheckbox {
		t.Fatalf("nested task projection = %q %#v", value, runs)
	}
	outdented, _, changed, handled := adjustNoteListIndent(indented, indentedRanges, woxui.TextSelection{Anchor: indentedRanges[2].TextEnd, Focus: indentedRanges[2].TextEnd}, -1)
	if !handled || !changed || outdented.Blocks[2].Indent != 1 {
		t.Fatalf("outdented task = %#v", outdented.Blocks[2])
	}
}
