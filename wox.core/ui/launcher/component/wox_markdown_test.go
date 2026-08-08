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
	}, 690, new(int))
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
	document := ParseMarkdown(`https://wox.one`)
	opened := ""
	widget := renderMarkdownBlock(document.blocks[0], MarkdownProps{
		ID: "preview", Theme: Theme{Cursor: woxui.Color{A: 255}}, OnOpenLink: func(target string) { opened = target },
	}, 300, new(int))

	wrap := widget.(woxwidget.Wrap)
	semantics := wrap.Children[0].(woxwidget.Semantics)
	focusable := semantics.Child.(woxwidget.Focusable)
	gesture := focusable.Child.(woxwidget.Gesture)
	gesture.OnTap()

	if opened != "https://wox.one" {
		t.Fatalf("opened link = %q, want https://wox.one", opened)
	}
	text := gesture.Child.(woxwidget.Text)
	if text.Style.Size != 12 || !text.Underline {
		t.Fatalf("link style = %#v, want Flutter preview size 12 with underline", text)
	}
}

func TestMarkdownLinkCanExcludeKeyboardFocus(t *testing.T) {
	document := ParseMarkdown(`[Install](https://wox.one)`)
	widget := renderMarkdownBlock(document.blocks[0], MarkdownProps{
		ID: "form-help", ExcludeLinkFocus: true, Theme: Theme{Cursor: woxui.Color{A: 255}}, OnOpenLink: func(string) {},
	}, 300, new(int))
	wrap := widget.(woxwidget.Wrap)
	semantics := wrap.Children[0].(woxwidget.Semantics)
	if _, ok := semantics.Child.(woxwidget.Focusable); ok {
		t.Fatal("form help markdown links should match Flutter ExcludeFocus")
	}
	if _, ok := semantics.Child.(woxwidget.Gesture); !ok {
		t.Fatalf("excluded link child = %T, want pointer Gesture", semantics.Child)
	}
}
