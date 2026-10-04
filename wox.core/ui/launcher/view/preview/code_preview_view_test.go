package preview

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestCodePreviewPreservesSourceAndWrappedLineNumbers(t *testing.T) {
	const source = "// 中文 comment\n\npackage main\nvar message = `a long string that wraps in a narrow preview and keeps its original line number`\n"
	tokens, err := chroma.Tokenise(lexers.Get("go"), nil, source)
	if err != nil {
		t.Fatal(err)
	}
	for _, scale := range []float32{0.9, 1, 1.1, 1.5} {
		for _, width := range []float32{160, 420} {
			theme := woxcomponent.Theme{Controls: woxcomponent.ControlTheme{DensityScale: scale}, PreviewText: woxui.Color{R: 240, G: 240, B: 240, A: 255}}
			window := &woxui.Window{}
			view := CodePreview(CodePreviewProps{ID: "source", Value: source, Tokens: tokens, Note: "Limited preview", Width: width, Height: 240, Theme: theme, Window: window}).(woxwidget.Container)
			scroll := resolvedScrollViewProps(view.Child, woxui.Size{Width: width - view.Padding.Left*2, Height: 240 - view.Padding.Top*2})
			body := scroll.Content.(woxwidget.Flex)
			row := body.Children[0].(woxwidget.Flex)
			field := row.Children[1].(woxwidget.Stateful).Widget.(woxcomponent.TextFieldProps)
			contentWidth := row.Children[0].(woxwidget.Container).Width + row.Gap + field.Width
			if !scroll.ReserveScrollbarSpace || contentWidth != scroll.ContentViewportWidth() || body.Children[1].(woxwidget.TextBlock).Width != contentWidth {
				t.Fatalf("code and note must fit beside the scrollbar: content=%v viewport=%v", contentWidth, scroll.ContentViewportWidth())
			}
			if !field.ReadOnly || field.Value != source || field.Style.Family != woxui.FontFamilyMonospace || field.Style.Size != theme.Controls.Scaled(13) {
				t.Fatalf("code field lost source, read-only behavior, or scaled monospace: %#v", field)
			}
			numbers := row.Children[0].(woxwidget.Container).Child.(woxwidget.Flex).Children
			if len(numbers) != strings.Count(source, "\n")+1 || len(body.Children) != 2 {
				t.Fatal("source lines, blank lines and the unnumbered limit note must stay separate")
			}
			var gutterHeight float32
			for _, number := range numbers {
				gutterHeight += number.(woxwidget.Container).Height
			}
			count := woxcomponent.TextFieldVisualLineCount(source, window, field.Style, field.Width, field.RichRuns)
			if gutterHeight != float32(count)*field.LineHeight || field.Height != gutterHeight+1 {
				t.Fatalf("scale %v width %v: gutter height %v differs from %d text lines at %v", scale, width, gutterHeight, count, field.LineHeight)
			}
		}
	}
}

func TestCodePreviewSyntaxUsesRuneOffsetsAndLightDarkInk(t *testing.T) {
	const source = "// 中文\npackage main\nvar s = \"hello\"\nvar n = 42"
	tokens, err := chroma.Tokenise(lexers.Get("go"), nil, source)
	if err != nil {
		t.Fatal(err)
	}
	dark := codePreviewRuns(tokens, woxui.Color{R: 240, G: 240, B: 240, A: 255}, utf8.RuneCountInString(source))
	light := codePreviewRuns(tokens, woxui.Color{R: 30, G: 30, B: 30, A: 255}, utf8.RuneCountInString(source))
	if len(dark) == 0 || len(light) != len(dark) {
		t.Fatal("expected syntax colors for both appearances")
	}
	seen := map[string]bool{}
	for index, run := range dark {
		text := string([]rune(source)[run.Start:run.End])
		seen[text] = true
		if run.Start != light[index].Start || run.End != light[index].End || run.Color == light[index].Color {
			t.Fatalf("light/dark styling changed offsets or reused the same ink for %q", text)
		}
	}
	for _, text := range []string{"// 中文", "package", "var", "\"hello\"", "42"} {
		if !seen[text] {
			t.Fatalf("missing syntax run %q: %#v", text, seen)
		}
	}
}
