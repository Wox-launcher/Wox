package component

import (
	"strings"
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestParseMarkdownBuildsSharedPreviewBlocks(t *testing.T) {
	document := ParseMarkdown("# Title\n\nParagraph with **bold** and [link](https://wox.one).\n\n- [x] done\n\n> quote\n\n```go\nfmt.Println(1)\n```\n\n| A | B |\n| - | - |\n| 1 | 2 |\n\n![[/tmp/example.png|Example]]")
	if len(document.blocks) != 7 {
		t.Fatalf("block count = %d, want 7", len(document.blocks))
	}
	wants := []markdownBlockKind{markdownHeading, markdownParagraph, markdownList, markdownQuote, markdownCode, markdownTable, markdownImage}
	for index, want := range wants {
		if document.blocks[index].kind != want {
			t.Fatalf("block %d kind = %d, want %d", index, document.blocks[index].kind, want)
		}
	}
	if document.blocks[2].items[0].marker != "" {
		t.Fatalf("task list marker = %q, want empty", document.blocks[2].items[0].marker)
	}
	if !document.blocks[2].items[0].task || !document.blocks[2].items[0].checked || document.blocks[2].items[0].label != "done" || document.blocks[2].items[0].blocks[0].runs[0].text != "done" {
		t.Fatalf("task list item = %#v, want checked task metadata without emoji text", document.blocks[2].items[0])
	}
	if document.blocks[4].language != "go" {
		t.Fatalf("code language = %q, want go", document.blocks[4].language)
	}
	if document.blocks[5].table.headerRows != 1 || len(document.blocks[5].table.rows) != 2 {
		t.Fatalf("table = %#v, want one header and two rows", document.blocks[5].table)
	}
	if document.blocks[6].image != "/tmp/example.png" || document.blocks[6].imageLabel != "Example" {
		t.Fatalf("image = %#v, want normalized wiki image", document.blocks[6])
	}
}

func TestMarkdownUsesSharedDocumentDecorations(t *testing.T) {
	theme := Theme{
		Cursor:         woxui.Color{R: 30, G: 120, B: 220, A: 255},
		PreviewText:    woxui.Color{R: 230, G: 230, B: 230, A: 255},
		PreviewSplit:   woxui.Color{R: 90, G: 90, B: 90, A: 255},
		ResultSubtitle: woxui.Color{R: 140, G: 140, B: 140, A: 255},
	}
	document := ParseMarkdown("- [x] done\n\n> quote\n\n---")

	list := renderMarkdownBlock(document.blocks[0], MarkdownProps{Theme: theme}, 300, new(int), new(int)).(woxwidget.Flex)
	row := list.Children[0].(woxwidget.Flex)
	marker := row.Children[0].(woxwidget.Container).Child.(woxwidget.Semantics)
	if marker.Role != woxui.AccessibilityRoleCheckBox || !marker.Checked || !marker.Disabled {
		t.Fatalf("task marker semantics = %#v, want disabled checked checkbox", marker)
	}
	if painter, ok := marker.Child.(woxwidget.Painter); !ok || painter.Width != documentCheckboxWidth(12) || painter.Height != 18 {
		t.Fatalf("task marker = %#v, want shared document checkbox", marker.Child)
	}
	body := row.Children[1].(woxwidget.Container).Child.(woxwidget.Flex).Children[0].(woxwidget.Wrap)
	if text := body.Children[0].(woxwidget.Text); text.Color != theme.ResultSubtitle || !text.Strike {
		t.Fatalf("completed task text = %#v, want muted %#v with strikethrough", text, theme.ResultSubtitle)
	}
	bullet := renderMarkdownBlock(ParseMarkdown("- item").blocks[0], MarkdownProps{Theme: theme}, 300, new(int), new(int)).(woxwidget.Flex)
	bulletMarker := bullet.Children[0].(woxwidget.Flex).Children[0].(woxwidget.Container).Child.(woxwidget.Text)
	if bulletMarker.Value != "•" || bulletMarker.Color != DocumentListMarkerColor {
		t.Fatalf("list marker = %#v, want fixed #1379D2", bulletMarker)
	}

	quote := renderMarkdownBlock(document.blocks[1], MarkdownProps{Theme: theme}, 300, new(int), new(int)).(woxwidget.Container).Child.(woxwidget.Container)
	if quote.LeftBorderColor != DocumentListMarkerColor || quote.LeftBorderWidth != documentQuoteBarWidth*documentDecorationScale(12) {
		t.Fatalf("quote bar = %.1f/%#v, want shared #1379D2 accent bar", quote.LeftBorderWidth, quote.LeftBorderColor)
	}
	rule := renderMarkdownBlock(document.blocks[2], MarkdownProps{Theme: theme}, 300, new(int), new(int)).(woxwidget.Painter)
	if rule.Width != 300 || rule.Height != documentRuleHeight {
		t.Fatalf("rule = %.0fx%.0f, want shared full-width rule", rule.Width, rule.Height)
	}
}

func TestParseMarkdownNestsUnderIndentedOrderedLists(t *testing.T) {
	document := ParseMarkdown("Create an app:\n1. **App Name**: Wox\n2. **App description**: Wox spotify plugin\n3. **Website**: https://github.com/Wox-launcher/Wox\n4. **Redirect URIs**:\n  1. wox://plugin/aeb94d3d-9c39-4917-9cd0-a4cde95433a2?action=spotify-access-token\n  2. wox://plugin/aeb94d3d-9c39-4917-9cd0-a4cde95433a2?action=spotify-auth\n5. **Which API/SDKs are you planning to use**: Web API")
	if len(document.blocks) != 2 || document.blocks[1].kind != markdownList {
		t.Fatalf("blocks = %#v, want an intro paragraph and one list", document.blocks)
	}
	items := document.blocks[1].items
	if len(items) != 5 {
		t.Fatalf("top-level items = %d (%#v), want 5 so Redirect URIs stay nested", len(items), items)
	}
	if items[3].marker != "4." || items[4].marker != "5." {
		t.Fatalf("markers = %q %q, want 4. then 5.", items[3].marker, items[4].marker)
	}
	nested := markdownNestedList(items[3].blocks)
	if nested == nil || len(nested.items) != 2 {
		t.Fatalf("redirect children = %#v, want two nested URI items", items[3].blocks)
	}
	if !strings.Contains(nested.items[0].label, "spotify-access-token") || !strings.Contains(nested.items[1].label, "spotify-auth") {
		t.Fatalf("nested URIs = %#v", nested.items)
	}
	if nested.items[0].marker != "a." || nested.items[1].marker != "b." {
		t.Fatalf("nested markers = %q %q, want a. then b.", nested.items[0].marker, nested.items[1].marker)
	}
}

func TestMarkdownOrderedMarkersChangeByNestingDepth(t *testing.T) {
	if markdownOrderedMarker(0, 4) != "4." || markdownOrderedMarker(1, 1) != "a." || markdownOrderedMarker(1, 27) != "aa." {
		t.Fatalf("decimal/alpha markers = %q %q %q", markdownOrderedMarker(0, 4), markdownOrderedMarker(1, 1), markdownOrderedMarker(1, 27))
	}
	if markdownOrderedMarker(2, 2) != "ii." || markdownOrderedMarker(2, 4) != "iv." || markdownOrderedMarker(3, 1) != "A." {
		t.Fatalf("roman/upper markers = %q %q %q", markdownOrderedMarker(2, 2), markdownOrderedMarker(2, 4), markdownOrderedMarker(3, 1))
	}
	document := ParseMarkdown("1. one\n   1. two\n      1. three\n         1. four")
	first := document.blocks[0].items[0]
	second := markdownNestedList(first.blocks).items[0]
	third := markdownNestedList(second.blocks).items[0]
	fourth := markdownNestedList(third.blocks).items[0]
	if first.marker != "1." || second.marker != "a." || third.marker != "i." || fourth.marker != "A." {
		t.Fatalf("nested outline = %q %q %q %q", first.marker, second.marker, third.marker, fourth.marker)
	}
}

func markdownNestedList(blocks []markdownBlock) *markdownBlock {
	for index := range blocks {
		if blocks[index].kind == markdownList {
			return &blocks[index]
		}
	}
	return nil
}

func TestNormalizeMarkdownListIndentSkipsFencedCode(t *testing.T) {
	source := "```\n1. keep\n  1. also keep\n```"
	if got := normalizeMarkdownListIndent(source); got != source {
		t.Fatalf("fenced list indent = %q, want unchanged", got)
	}
}

func TestNestedMarkdownTaskStateStaysOnTheOwningListItem(t *testing.T) {
	document := ParseMarkdown("- parent\n  - [x] child")
	parent := document.blocks[0].items[0]
	if parent.task {
		t.Fatal("parent list item inherited its nested task state")
	}
	if len(parent.blocks) < 2 || len(parent.blocks[1].items) != 1 || !parent.blocks[1].items[0].task || !parent.blocks[1].items[0].checked {
		t.Fatalf("nested task = %#v, want checked state on child only", parent.blocks)
	}
}

func TestParseMarkdownPromotesImageOnSoftLineBreak(t *testing.T) {
	document := ParseMarkdown("Intro paragraph\n![](https://example.com/shot.png)")
	if len(document.blocks) != 2 || document.blocks[0].kind != markdownParagraph || document.blocks[1].kind != markdownImage {
		t.Fatalf("blocks = %#v, want paragraph followed by image", document.blocks)
	}
	if document.blocks[1].image != "https://example.com/shot.png" {
		t.Fatalf("image = %q", document.blocks[1].image)
	}
}

func TestMarkdownImageUsesAvailableWidthWithoutHeightCap(t *testing.T) {
	document := ParseMarkdown("![](https://example.com/shot.png)")
	widget := renderMarkdownBlock(document.blocks[0], MarkdownProps{
		ID: "preview", ResolveImage: func(string) (*woxui.Image, string) { return &woxui.Image{Width: 2560, Height: 1788}, "" },
	}, 690, new(int), new(int))
	align := widget.(woxwidget.Align)
	image := align.Child.(woxwidget.Image)
	if image.Width != 690 || image.Height <= 280 {
		t.Fatalf("image size = %.0fx%.0f, want full width without 280px cap", image.Width, image.Height)
	}
}

func TestParseMarkdownRejectsUnsafeLinksWithoutDroppingText(t *testing.T) {
	document := ParseMarkdown(`[safe \[label\]](https://wox.one) [unsafe](javascript:alert(1))`)
	if len(document.blocks) != 1 {
		t.Fatalf("block count = %d, want 1", len(document.blocks))
	}
	var safeFound, unsafeTextFound bool
	for _, run := range document.blocks[0].runs {
		if strings.TrimSpace(run.text) == "safe [label]" && run.style.link == "https://wox.one" {
			safeFound = true
		}
		if strings.TrimSpace(run.text) == "unsafe" && run.style.link == "" {
			unsafeTextFound = true
		}
	}
	if !safeFound || !unsafeTextFound {
		t.Fatalf("runs = %#v, want safe link and plain unsafe label", document.blocks[0].runs)
	}
}

func TestMarkdownLinkOpensFromPointerAction(t *testing.T) {
	document := ParseMarkdown(`[#4497](https://github.com/Wox-launcher/Wox/issues/4497)`)
	opened := ""
	widget := renderMarkdownBlock(document.blocks[0], MarkdownProps{
		ID: "preview", Theme: Theme{Cursor: woxui.Color{R: 255, G: 255, B: 255, A: 255}}, OnOpenLink: func(target string) { opened = target },
	}, 300, new(int), new(int))

	wrap := widget.(woxwidget.Wrap)
	semantics := wrap.Children[0].(woxwidget.Semantics)
	focusable := semantics.Child.(woxwidget.Focusable)
	gesture := focusable.Child.(woxwidget.Gesture)
	gesture.OnTap()

	if opened != "https://github.com/Wox-launcher/Wox/issues/4497" {
		t.Fatalf("opened link = %q, want the Wox issue URL", opened)
	}
	if gesture.Cursor != woxui.PointerCursorHand {
		t.Fatalf("link cursor = %v, want hand", gesture.Cursor)
	}
	text := gesture.Child.(woxwidget.Text)
	if text.Value != "#4497" || text.Style.Size != 12 || !text.Underline || text.Color != DocumentListMarkerColor {
		t.Fatalf("link style = %#v, want the issue label with document accent and underline", text)
	}
	opened = ""
	if !focusable.OnKey(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true}) || opened != "https://github.com/Wox-launcher/Wox/issues/4497" {
		t.Fatalf("keyboard activation opened %q, want the Wox issue URL", opened)
	}
}

func TestMarkdownLinkCanExcludeKeyboardFocus(t *testing.T) {
	document := ParseMarkdown(`[Install](https://wox.one)`)
	widget := renderMarkdownBlock(document.blocks[0], MarkdownProps{
		ID: "form-help", ExcludeLinkFocus: true, Theme: Theme{Cursor: woxui.Color{A: 255}}, OnOpenLink: func(string) {},
	}, 300, new(int), new(int))
	wrap := widget.(woxwidget.Wrap)
	semantics := wrap.Children[0].(woxwidget.Semantics)
	if _, ok := semantics.Child.(woxwidget.Focusable); ok {
		t.Fatal("form help markdown links should match Flutter ExcludeFocus")
	}
	if _, ok := semantics.Child.(woxwidget.Gesture); !ok {
		t.Fatalf("excluded link child = %T, want pointer Gesture", semantics.Child)
	}
}

func TestMarkdownTableUsesCollapsedGridLines(t *testing.T) {
	theme := Theme{PreviewSplit: woxui.Color{R: 90, G: 90, B: 90, A: 255}, PreviewText: woxui.Color{A: 255}}
	widget := renderMarkdownBlock(ParseMarkdown("| A | B |\n| - | - |\n| 1 | 2 |").blocks[0], MarkdownProps{Theme: theme}, 300, new(int), new(int)).(woxwidget.Stack)
	if len(widget.Children) != 2 {
		t.Fatalf("markdown table children = %d, want content plus one outer stroke", len(widget.Children))
	}
	outline := widget.Children[1].Child.(woxwidget.Container)
	if outline.BorderWidth != TableGridBorderWidth || outline.Color.A != 0 {
		t.Fatalf("markdown table stroke = %#v, want the shared 1px frame", outline)
	}
	rows := widget.Children[0].Child.(woxwidget.ScrollView).Child.(woxwidget.Flex)
	header := rows.Children[0].(woxwidget.Flex).Children[0].(woxwidget.Container)
	if header.BorderWidth != 0 || header.RightBorderWidth != TableGridBorderWidth || header.BottomBorderWidth != TableGridBorderWidth {
		t.Fatalf("markdown header cell = %#v, want collapsed right+bottom", header)
	}
	last := rows.Children[1].(woxwidget.Flex).Children[1].(woxwidget.Container)
	if last.RightBorderWidth != 0 || last.BottomBorderWidth != 0 {
		t.Fatalf("markdown last cell = %#v, want the outer frame to own that corner", last)
	}
}

func TestMarkdownLinkUsesHandCursor(t *testing.T) {
	_, _, links := markdownRunsContent(ParseMarkdown("[Dashboard](https://developer.spotify.com/dashboard)").blocks[0].runs, 12, Theme{}, false)
	if markdownCursorAt(links, 0) != woxui.PointerCursorHand {
		t.Fatal("hovering a Markdown link should use the hand cursor")
	}
	if markdownCursorAt(links, len("Dashboard")) != woxui.PointerCursorText {
		t.Fatal("text after a Markdown link should keep the text cursor")
	}
}

func TestWoxMarkdownUsesSelectableTextWhenWindowIsSet(t *testing.T) {
	const body = "Copy this and that."
	widget := WoxMarkdown(MarkdownProps{
		ID: "md", Document: ParseMarkdown("Copy **this** and [that](https://wox.one)."),
		Width: 300, Window: &woxui.Window{}, Theme: Theme{PreviewText: woxui.Color{A: 255}},
		OnOpenLink: func(string) {},
	})
	flex := widget.(woxwidget.Flex)
	field, ok := flex.Children[0].(woxwidget.Stateful)
	if !ok {
		t.Fatalf("child = %T, want a read-only text field for selection", flex.Children[0])
	}
	props := field.Widget.(TextFieldProps)
	if props.ID != "md-text-1" || props.Label != body || props.Value != body || !props.ReadOnly {
		t.Fatalf("selectable markdown field = %#v, want labeled read-only text %q", props, body)
	}
}

func TestWoxMarkdownSelectAllCopiesPlainText(t *testing.T) {
	provider := &memoryClipboard{}
	SetClipboardProvider(provider)
	t.Cleanup(func() { SetClipboardProvider(nil) })

	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return WoxMarkdown(MarkdownProps{
			ID: "md", Document: ParseMarkdown("Copy **this** and [that](https://wox.one)."),
			Width: 300, Window: &woxui.Window{}, Theme: Theme{PreviewText: woxui.Color{A: 255}},
			OnOpenLink: func(string) {},
		})
	})
	host.AttachServices(&hotkeyRecorderHostServices{})
	displayList := &woxui.DisplayList{}
	host.Frame(displayList, woxui.FrameInfo{Size: woxui.Size{Width: 300, Height: 80}, PixelSize: woxui.PixelSize{Width: 300, Height: 80}, Scale: 1})
	if diagnostics := host.Snapshot().Diagnostics; len(diagnostics) > 0 {
		t.Fatalf("selectable markdown semantics diagnostics: %v", diagnostics)
	}
	host.RequestFocus("md-text-1")
	primary := woxui.KeyModifierControl | woxui.KeyModifierMeta
	if !host.Key(woxui.KeyEvent{Key: woxui.Key("a"), Modifiers: primary, Down: true}) {
		t.Fatal("Ctrl/Cmd+A should select the markdown text")
	}
	if !host.Key(woxui.KeyEvent{Key: woxui.Key("c"), Modifiers: primary, Down: true}) {
		t.Fatal("Ctrl/Cmd+C should copy the selected markdown text")
	}
	if provider.text != "Copy this and that." {
		t.Fatalf("copied markdown = %q, want the rendered plain text", provider.text)
	}
}
