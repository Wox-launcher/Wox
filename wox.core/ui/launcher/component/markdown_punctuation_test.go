package component

import "testing"

func TestMarkdownAndNoteEditorRenderPunctuationAdjacentBold(t *testing.T) {
	const source = "**通常不行。**孩子"
	document := ParseMarkdown(source)
	if len(document.blocks) != 1 || len(document.blocks[0].runs) != 2 || document.blocks[0].runs[0].text != "通常不行。" || !document.blocks[0].runs[0].style.bold || document.blocks[0].runs[1].text != "孩子" || document.blocks[0].runs[1].style.bold {
		t.Fatalf("Markdown runs = %#v", document.blocks)
	}
	text, spans := parseNoteInlineMarkdown(source, nil)
	if text != "通常不行。孩子" || len(spans) != 1 || !spans[0].Bold || spans[0].Start != 0 || spans[0].End != 5 {
		t.Fatalf("note inline text = %q, spans = %#v", text, spans)
	}
}
