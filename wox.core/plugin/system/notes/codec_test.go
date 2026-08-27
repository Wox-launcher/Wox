package notes

import (
	"strings"
	"testing"

	"wox/common"
)

func TestNoteCodecsPreserveSupportedFormattingAndEscapeHTML(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{Type: common.NoteBlockHeading1, Text: "Hello <Wox>", Spans: []common.NoteSpan{{Start: 0, End: 5, Bold: true}}},
		{Type: common.NoteBlockTask, Text: "ship notes", Checked: true, Spans: []common.NoteSpan{{Start: 5, End: 10, Strike: true}}},
		{Type: common.NoteBlockQuote, Text: "中文"},
	}}
	markdown := ToMarkdown(document)
	if !strings.Contains(markdown, "# **Hello** <Wox>") || !strings.Contains(markdown, "- [x]") {
		t.Fatalf("unexpected markdown: %s", markdown)
	}
	htmlValue := ToHTML(document)
	if strings.Contains(htmlValue, "<Wox>") || !strings.Contains(htmlValue, "&lt;Wox&gt;") || !strings.Contains(htmlValue, "<strong>Hello</strong>") {
		t.Fatalf("unexpected html: %s", htmlValue)
	}
	plain := ToPlainText(document)
	if !strings.Contains(plain, "☑ ship notes") || !strings.Contains(plain, "> 中文") {
		t.Fatalf("unexpected plain text: %s", plain)
	}
}

func TestParseMarkdownBuildsRichBlocks(t *testing.T) {
	document := ParseMarkdown("# **Title**\n- [x] done\n- item\n\n> quote")
	if len(document.Blocks) != 4 {
		t.Fatalf("block count = %d, want 4: %#v", len(document.Blocks), document.Blocks)
	}
	if document.Blocks[0].Type != common.NoteBlockHeading1 || len(document.Blocks[0].Spans) == 0 || !document.Blocks[0].Spans[0].Bold {
		t.Fatalf("heading formatting not preserved: %#v", document.Blocks[0])
	}
	if document.Blocks[1].Type != common.NoteBlockTask || !document.Blocks[1].Checked || document.Blocks[2].Type != common.NoteBlockBullet {
		t.Fatalf("list types not preserved: %#v", document.Blocks[1:3])
	}
	if document.Blocks[3].Type != common.NoteBlockQuote {
		t.Fatalf("quote type = %s", document.Blocks[3].Type)
	}
}

func TestParseMarkdownKeepsMultilineCodeEditableAndExportsOneFence(t *testing.T) {
	document := ParseMarkdown("```go\nfirst()\nsecond()\n```")
	if len(document.Blocks) != 2 || document.Blocks[0].Type != common.NoteBlockCode || document.Blocks[1].Type != common.NoteBlockCode {
		t.Fatalf("multiline code was not normalized into editable lines: %#v", document.Blocks)
	}
	if got := ToMarkdown(document); got != "```\nfirst()\nsecond()\n```" {
		t.Fatalf("unexpected multiline code export: %q", got)
	}
}

func TestNormalizeDocumentClampsInvalidSpansAndRepairsEmptyDocument(t *testing.T) {
	document := NormalizeDocument(common.NoteDocument{Blocks: []common.NoteBlock{{Text: "中a", Spans: []common.NoteSpan{{Start: -3, End: 99, Italic: true}, {Start: 1, End: 1, Bold: true}}}}})
	if document.Version != documentVersion || document.Blocks[0].ID == "" || document.Blocks[0].Type != common.NoteBlockParagraph {
		t.Fatalf("document was not repaired: %#v", document)
	}
	if len(document.Blocks[0].Spans) != 1 || document.Blocks[0].Spans[0].Start != 0 || document.Blocks[0].Spans[0].End != 2 {
		t.Fatalf("spans were not normalized: %#v", document.Blocks[0].Spans)
	}
	if len(NormalizeDocument(common.NoteDocument{}).Blocks) != 1 {
		t.Fatal("empty document should receive one paragraph")
	}
}

func TestHTMLExportRejectsExecutableLinkSchemes(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{{Text: "click", Spans: []common.NoteSpan{{Start: 0, End: 5, Link: "javascript:alert(1)"}}}}}
	value := ToHTML(document)
	if strings.Contains(value, "javascript:") || strings.Contains(value, "<a ") {
		t.Fatalf("unsafe link was exported: %s", value)
	}
}

func TestDocumentIsEmptyIgnoresBlankBlocksAndKeepsStructuralContent(t *testing.T) {
	if !DocumentIsEmpty(EmptyDocument()) || !DocumentIsEmpty(common.NoteDocument{Blocks: []common.NoteBlock{{Type: common.NoteBlockHeading1, Text: "  "}, {Type: common.NoteBlockTask}}}) {
		t.Fatal("blank notes should be empty")
	}
	if DocumentIsEmpty(common.NoteDocument{Blocks: []common.NoteBlock{{Text: "hello"}}}) || DocumentIsEmpty(common.NoteDocument{Blocks: []common.NoteBlock{{Type: common.NoteBlockDivider}}}) {
		t.Fatal("typed or divider notes should be kept")
	}
}

func TestParseMarkdownKeepsGFMTables(t *testing.T) {
	document := ParseMarkdown("| A | **B** |\n| --- | --- |\n| 1 | 2 |")
	if len(document.Blocks) != 1 || document.Blocks[0].Type != common.NoteBlockTable || document.Blocks[0].Table == nil {
		t.Fatalf("table was not parsed: %#v", document.Blocks)
	}
	table := document.Blocks[0].Table
	if table.HeaderRows != 1 || len(table.Rows) != 2 || table.Rows[0][0].Text != "A" || table.Rows[1][1].Text != "2" {
		t.Fatalf("table cells = %#v", table)
	}
	if len(table.Rows[0][1].Spans) == 0 || !table.Rows[0][1].Spans[0].Bold {
		t.Fatalf("header inline style lost: %#v", table.Rows[0][1])
	}
	markdown := ToMarkdown(document)
	if !strings.Contains(markdown, "| A |") || !strings.Contains(markdown, "| --- |") || !strings.Contains(markdown, "| 1 |") {
		t.Fatalf("table markdown = %q", markdown)
	}
	if !strings.Contains(ToHTML(document), "<table>") || !strings.Contains(ToHTML(document), "<th>") {
		t.Fatalf("table html = %s", ToHTML(document))
	}
	if ToPlainText(document) != "A\tB\n1\t2" && !strings.Contains(ToPlainText(document), "A") {
		t.Fatalf("table plain text = %q", ToPlainText(document))
	}
}

func TestParseHTMLAndTSVBuildTables(t *testing.T) {
	htmlDocument := ParseHTML(`<article><table><tr><th>Name</th><th>Age</th></tr><tr><td>Ada</td><td>36</td></tr></table></article>`)
	if len(htmlDocument.Blocks) != 1 || htmlDocument.Blocks[0].Type != common.NoteBlockTable || htmlDocument.Blocks[0].Table.Rows[0][0].Text != "Name" {
		t.Fatalf("html table = %#v", htmlDocument.Blocks)
	}
	tsv := ParseClipboard("Name\tAge\nAda\t36")
	if len(tsv.Blocks) != 1 || tsv.Blocks[0].Type != common.NoteBlockTable || tsv.Blocks[0].Table.Rows[1][0].Text != "Ada" {
		t.Fatalf("tsv table = %#v", tsv.Blocks)
	}
}

func TestNormalizeDocumentUpgradesPipeTableParagraphs(t *testing.T) {
	document := NormalizeDocument(common.NoteDocument{Blocks: []common.NoteBlock{{
		Type: common.NoteBlockParagraph, Text: "| A | B |\n| --- | --- |\n| 1 | 2 |",
	}}})
	if len(document.Blocks) != 1 || document.Blocks[0].Type != common.NoteBlockTable || document.Blocks[0].Table == nil {
		t.Fatalf("legacy pipe table was not upgraded: %#v", document.Blocks)
	}
	if !strings.Contains(document.Blocks[0].Text, "| A |") {
		t.Fatalf("table fallback text missing: %q", document.Blocks[0].Text)
	}
}

func TestToMarkdownRoundTripsTableAfterParagraph(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{Type: common.NoteBlockParagraph, Text: "intro"},
		{Type: common.NoteBlockTable, Table: &common.NoteTable{HeaderRows: 1, Rows: [][]common.NoteTableCell{
			{{Text: "A"}, {Text: "B"}},
			{{Text: "1"}, {Text: "2"}},
		}}},
	}}
	markdown := ToMarkdown(document)
	if !strings.Contains(markdown, "intro\n\n|") {
		t.Fatalf("table markdown must leave a blank line after the paragraph: %q", markdown)
	}
	parsed := ParseMarkdown(markdown)
	if len(parsed.Blocks) != 2 || parsed.Blocks[0].Text != "intro" || parsed.Blocks[1].Type != common.NoteBlockTable || parsed.Blocks[1].Table == nil {
		t.Fatalf("round-tripped table = %#v from %q", parsed.Blocks, markdown)
	}
	if parsed.Blocks[1].Table.Rows[0][0].Text != "A" || parsed.Blocks[1].Table.Rows[1][1].Text != "2" {
		t.Fatalf("round-tripped cells = %#v", parsed.Blocks[1].Table)
	}
}

func TestNormalizeDocumentExtractsTableGluedToParagraph(t *testing.T) {
	document := NormalizeDocument(common.NoteDocument{Blocks: []common.NoteBlock{{
		Type: common.NoteBlockParagraph, Text: "intro\n| A | B |\n| --- | --- |\n| 1 | 2 |\nafter",
	}}})
	if len(document.Blocks) != 3 || document.Blocks[0].Text != "intro" || document.Blocks[1].Type != common.NoteBlockTable || document.Blocks[2].Text != "after" {
		t.Fatalf("glued table = %#v", document.Blocks)
	}
	if document.Blocks[1].Table == nil || document.Blocks[1].Table.Rows[1][1].Text != "2" {
		t.Fatalf("extracted cells = %#v", document.Blocks[1].Table)
	}
}

func TestDocumentIsEmptyKeepsTables(t *testing.T) {
	if DocumentIsEmpty(common.NoteDocument{Blocks: []common.NoteBlock{{Type: common.NoteBlockTable, Table: &common.NoteTable{Rows: [][]common.NoteTableCell{{{Text: "A"}}}}}}}) {
		t.Fatal("tables should keep a note from being empty")
	}
}

func TestDocumentIsEmptyKeepsImages(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{{
		Type: common.NoteBlockImage, Image: &common.NoteImage{ID: "shot.png", FileName: "shot.png"},
	}}}
	if DocumentIsEmpty(document) {
		t.Fatal("image notes should be kept")
	}
	if CustomNoteTitle(document) != "shot" || NoteTitle(document) != "shot" {
		t.Fatalf("image title = %q", NoteTitle(document))
	}
}

func TestNoteImageCodecsRoundTripAttachmentRefs(t *testing.T) {
	document := NormalizeDocument(common.NoteDocument{Blocks: []common.NoteBlock{{
		Type: common.NoteBlockImage, Image: &common.NoteImage{ID: "abc.png", FileName: "capture.png"},
	}}})
	markdown := ToMarkdown(document)
	if !strings.Contains(markdown, "![capture.png](notes-image:abc.png)") {
		t.Fatalf("image markdown = %q", markdown)
	}
	parsed := ParseMarkdown(markdown)
	image := noteTestImageBlock(parsed)
	if image == nil || image.ID != "abc.png" {
		t.Fatalf("parsed image = %#v", parsed.Blocks)
	}
	html := ToHTML(document)
	if !strings.Contains(html, `data-notes-image="abc.png"`) || !strings.Contains(html, `alt="capture.png"`) {
		t.Fatalf("image html = %s", html)
	}
	if ToPlainText(document) != "capture.png" {
		t.Fatalf("image plain text = %q", ToPlainText(document))
	}
}

func TestNoteImageCodecsRoundTripDisplayScale(t *testing.T) {
	document := NormalizeDocument(common.NoteDocument{Blocks: []common.NoteBlock{{
		Type: common.NoteBlockImage, Image: &common.NoteImage{ID: "abc.png", FileName: "capture.png", Scale: 60},
	}}})
	markdown := ToMarkdown(document)
	if !strings.Contains(markdown, "![capture.png](notes-image:abc.png?scale=60)") {
		t.Fatalf("scaled image markdown = %q", markdown)
	}
	parsed := ParseMarkdown(markdown)
	if image := noteTestImageBlock(parsed); image == nil || image.Scale != 60 {
		t.Fatalf("parsed scaled image = %#v", parsed.Blocks)
	}
	html := ToHTML(document)
	if !strings.Contains(html, `data-notes-image-scale="60"`) {
		t.Fatalf("scaled image html = %s", html)
	}
}

func TestNoteImageCodecsRoundTripIntrinsicSize(t *testing.T) {
	document := NormalizeDocument(common.NoteDocument{Blocks: []common.NoteBlock{{
		Type: common.NoteBlockImage, Image: &common.NoteImage{ID: "abc.png", FileName: "capture.png", Width: 1920, Height: 1080, Scale: 60},
	}}})
	markdown := ToMarkdown(document)
	if !strings.Contains(markdown, "![capture.png](notes-image:abc.png?scale=60&width=1920&height=1080)") {
		t.Fatalf("sized image markdown = %q", markdown)
	}
	parsed := ParseMarkdown(markdown)
	if image := noteTestImageBlock(parsed); image == nil || image.Width != 1920 || image.Height != 1080 || image.Scale != 60 {
		t.Fatalf("parsed sized image = %#v", parsed.Blocks)
	}
	html := ToHTML(document)
	if !strings.Contains(html, `data-notes-image-width="1920"`) || !strings.Contains(html, `data-notes-image-height="1080"`) {
		t.Fatalf("sized image html = %s", html)
	}
}

func TestNoteCodecsPreserveNestedTaskLevels(t *testing.T) {
	document := common.NoteDocument{Blocks: []common.NoteBlock{
		{Type: common.NoteBlockTask, Text: "parent"},
		{Type: common.NoteBlockTask, Text: "child", Indent: 1},
		{Type: common.NoteBlockTask, Text: "grandchild", Checked: true, Indent: 2},
	}}
	markdown := ToMarkdown(document)
	if markdown != "- [ ] parent\n    - [ ] child\n        - [x] grandchild" {
		t.Fatalf("nested task markdown = %q", markdown)
	}
	parsed := ParseMarkdown(markdown)
	if len(parsed.Blocks) != 3 || parsed.Blocks[0].Indent != 0 || parsed.Blocks[1].Indent != 1 || parsed.Blocks[2].Indent != 2 || !parsed.Blocks[2].Checked {
		t.Fatalf("nested task parse = %#v", parsed.Blocks)
	}
	if plain := ToPlainText(document); plain != "☐ parent\n    ☐ child\n        ☑ grandchild" {
		t.Fatalf("nested task plain text = %q", plain)
	}
}

func TestEnsureNoteImageEditGapsAddsCaretParagraphs(t *testing.T) {
	document := EnsureNoteImageEditGaps(common.NoteDocument{Blocks: []common.NoteBlock{
		{ID: "img", Type: common.NoteBlockImage, Image: &common.NoteImage{ID: "shot.png"}},
	}})
	if len(document.Blocks) != 3 || document.Blocks[0].Type != common.NoteBlockParagraph || document.Blocks[1].Type != common.NoteBlockImage || document.Blocks[2].Type != common.NoteBlockParagraph {
		t.Fatalf("image gaps = %#v", document.Blocks)
	}
	again := EnsureNoteImageEditGaps(document)
	if len(again.Blocks) != 3 {
		t.Fatalf("image gaps should be idempotent: %#v", again.Blocks)
	}
}

func noteTestImageBlock(document common.NoteDocument) *common.NoteImage {
	for _, block := range document.Blocks {
		if block.Type == common.NoteBlockImage && block.Image != nil {
			return block.Image
		}
	}
	return nil
}
