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
		if blockType == common.NoteBlockQuote && (len(runs) == 0 || !runs[0].LeadingBar || runs[0].Color != woxcomponent.DocumentListMarkerColor || runs[0].End != ranges[0].TextEnd) {
			t.Fatalf("quote did not project a continuous #1379D2 leading bar: %#v", runs)
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
	document := common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "task", Type: common.NoteBlockTask, Text: "done", Checked: true}}}
	_, runs, ranges := projectNoteDocument(document, woxui.TextStyle{Size: 14}, woxcomponent.Theme{ResultSubtitle: muted, Cursor: woxui.Color{R: 40, G: 130, B: 230, A: 255}})
	if len(runs) != 2 || !runs[0].Checkbox || !runs[0].Checked || runs[0].Color != woxcomponent.DocumentListMarkerColor || runs[1].Color != muted {
		t.Fatalf("checked task color = %#v, want marker %#v and muted %#v", runs, woxcomponent.DocumentListMarkerColor, muted)
	}
	if index, ok := noteTaskAtOffset(document, ranges, ranges[0].Start); !ok || index != 0 {
		t.Fatal("task checkbox prefix was not clickable")
	}
	if _, ok := noteTaskAtOffset(document, ranges, ranges[0].TextStart+1); ok {
		t.Fatal("task text must keep normal caret behavior")
	}
}

func TestNotesActiveFormatsFollowCaretAndSelection(t *testing.T) {
	document := common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
		{ID: "p", Type: common.NoteBlockParagraph, Text: "plain underlined", Spans: []common.NoteSpan{{Start: 6, End: 16, Underline: true}}},
		{ID: "h", Type: common.NoteBlockHeading1, Text: "title"},
	}}
	_, _, ranges := projectNoteDocument(document, woxui.TextStyle{Size: 14}, woxcomponent.Theme{})
	if formats := noteActiveFormats(document, ranges, woxui.TextSelection{Anchor: ranges[0].TextStart + 8, Focus: ranges[0].TextStart + 8}); !formats["underline"] || formats["bold"] || formats["block"] {
		t.Fatalf("caret in underline = %#v", formats)
	}
	if formats := noteActiveFormats(document, ranges, woxui.TextSelection{Anchor: ranges[0].TextStart, Focus: ranges[0].TextStart}); formats["underline"] {
		t.Fatalf("caret in plain text = %#v", formats)
	}
	if formats := noteActiveFormats(document, ranges, woxui.TextSelection{Anchor: ranges[1].TextStart + 1, Focus: ranges[1].TextStart + 1}); !formats["block"] {
		t.Fatalf("caret in heading = %#v", formats)
	}
	if formats := noteActiveFormats(document, ranges, woxui.TextSelection{Anchor: ranges[0].TextStart, Focus: ranges[0].TextEnd}); formats["underline"] {
		t.Fatalf("mixed selection must not keep underline active: %#v", formats)
	}
}

func TestNotesLinkLooksClickableAndOpensFromText(t *testing.T) {
	linkColor := woxui.Color{R: 19, G: 121, B: 210, A: 255}
	document := common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
		{ID: "task", Type: common.NoteBlockTask, Text: "sadf more", Spans: []common.NoteSpan{{Start: 0, End: 4, Link: "https://wox.one"}}},
	}}
	_, runs, ranges := projectNoteDocument(document, woxui.TextStyle{Size: 14}, woxcomponent.Theme{Cursor: linkColor})
	found := false
	for _, run := range runs {
		if run.Start == ranges[0].TextStart && run.End == ranges[0].TextStart+4 && run.Underline && run.Color == linkColor {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("link run = %#v, want underlined Cursor color on sadf", runs)
	}
	if got := noteLinkAtOffset(document, ranges, ranges[0].TextStart); got != "https://wox.one" {
		t.Fatalf("link at label = %q", got)
	}
	if got := noteLinkAtOffset(document, ranges, ranges[0].TextStart+4); got != "https://wox.one" {
		t.Fatalf("link at exclusive label end = %q", got)
	}
	if got := noteLinkAtOffset(document, ranges, ranges[0].TextStart+5); got != "" {
		t.Fatalf("plain text next to link = %q, want empty", got)
	}
	if got := noteLinkAtOffset(document, ranges, ranges[0].Marker); got != "" {
		t.Fatalf("checkbox prefix must not be a link: %q", got)
	}
	if noteOpenableLink("javascript:alert(1)") != "" || noteOpenableLink("not-a-url") != "" {
		t.Fatal("unsafe link targets must not become clickable")
	}
	controller := newNotesWindowController(&App{palette: defaultPalette()}, common.NoteRecord{ID: "note", Document: document})
	controller.document, controller.blockRanges = document, ranges
	if controller.editorCursorAt(ranges[0].TextStart) != woxui.PointerCursorHand {
		t.Fatal("link hover must show a hand cursor")
	}
	if controller.editorCursorAt(ranges[0].TextStart+5) != woxui.PointerCursorText {
		t.Fatal("plain text next to a link must keep the text cursor")
	}
	if !controller.handleBlockTap(ranges[0].TextStart) {
		t.Fatal("clicking a link must consume the tap")
	}
}

func TestNotesListMarkersUseFixedAccent(t *testing.T) {
	document := common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{
		{ID: "bullet", Type: common.NoteBlockBullet, Text: "one"},
		{ID: "ordered", Type: common.NoteBlockOrdered, Text: "two"},
		{ID: "task", Type: common.NoteBlockTask, Text: "three"},
	}}
	_, runs, ranges := projectNoteDocument(document, woxui.TextStyle{Size: 14}, woxcomponent.Theme{Cursor: woxui.Color{R: 250, G: 250, B: 250, A: 255}})
	want := []struct {
		start    int
		checkbox bool
	}{
		{ranges[0].Marker, false},
		{ranges[1].Marker, false},
		{ranges[2].Marker, true},
	}
	for _, item := range want {
		found := false
		for _, run := range runs {
			if run.Start == item.start && run.Color == woxcomponent.DocumentListMarkerColor && run.Checkbox == item.checkbox {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing list marker at %d checkbox=%t in %#v", item.start, item.checkbox, runs)
		}
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
