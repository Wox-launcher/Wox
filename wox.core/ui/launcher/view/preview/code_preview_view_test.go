package preview

import (
	"strings"
	"testing"
	"unicode/utf8"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestCodePreviewPreservesSourceAndWrappedLineNumbers(t *testing.T) {
	const source = "// 中文 comment\n\npackage main\nvar message = `a long string that wraps in a narrow preview and keeps its original line number`\n"
	tokens := TokenizeCode("main.go", source)
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
	tokens := TokenizeCode("main.go", source)
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

func TestCodeSyntaxTokenizerLanguagesAndInvariants(t *testing.T) {
	for _, plain := range []string{"notes.txt", "output.log", "data.csv", "file.unknown"} {
		if IsCodePreviewFile(plain) {
			t.Fatalf("%q should not be treated as code", plain)
		}
	}

	cases := []struct {
		name      string
		source    string
		wantKinds map[string]CodeTokenKind
	}{
		{
			name:   "app.py",
			source: "# 注释\ndef greet(foo2):\n    s = \"\"\"multi\nline\"\"\"\n    return 3.14\n",
			wantKinds: map[string]CodeTokenKind{
				"# 注释":                    CodeTokenComment,
				"def":                     CodeTokenKeyword,
				"return":                  CodeTokenKeyword,
				"\"\"\"multi\nline\"\"\"": CodeTokenString,
				"3.14":                    CodeTokenNumber,
				"foo2":                    CodeTokenPlain,
			},
		},
		{
			name:   "index.ts",
			source: "/* block */ export const n = 0xff;\nconst msg = `hi \\`world\\``;\n",
			wantKinds: map[string]CodeTokenKind{
				"/* block */":      CodeTokenComment,
				"export":           CodeTokenKeyword,
				"const":            CodeTokenKeyword,
				"0xff":             CodeTokenNumber,
				"`hi \\`world\\``": CodeTokenString,
			},
		},
		{
			name:   "config.json",
			source: "{\"enabled\": true, \"count\": 10}\n",
			wantKinds: map[string]CodeTokenKind{
				"\"enabled\"": CodeTokenString,
				"true":        CodeTokenKeyword,
				"10":          CodeTokenNumber,
			},
		},
		{
			name:   "index.html",
			source: "<!-- hi --><div class=\"box\">It's 42</div>",
			wantKinds: map[string]CodeTokenKind{
				"<!-- hi -->": CodeTokenComment,
				"div":         CodeTokenKeyword,
				"\"box\"":     CodeTokenString,
			},
		},
		{
			name:   "query.sql",
			source: "-- q\nSELECT * FROM users WHERE id = 7;\n",
			wantKinds: map[string]CodeTokenKind{
				"-- q":   CodeTokenComment,
				"SELECT": CodeTokenKeyword,
				"FROM":   CodeTokenKeyword,
				"WHERE":  CodeTokenKeyword,
				"7":      CodeTokenNumber,
			},
		},
		{
			name:   "lib.rs",
			source: "fn foo<'a>(x: &'a str) -> char { 'z' }\n",
			wantKinds: map[string]CodeTokenKind{
				"fn":  CodeTokenKeyword,
				"'z'": CodeTokenString,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !IsCodePreviewFile(tc.name) {
				t.Fatalf("%q should be recognized as code", tc.name)
			}
			tokens := TokenizeCode(tc.name, tc.source)
			var rebuilt strings.Builder
			got := map[string]CodeTokenKind{}
			for _, tok := range tokens {
				rebuilt.WriteString(tok.Value)
				got[tok.Value] = tok.Kind
			}
			if rebuilt.String() != tc.source {
				t.Fatalf("roundtrip mismatch: got %q, want %q", rebuilt.String(), tc.source)
			}
			for val, wantKind := range tc.wantKinds {
				if wantKind == CodeTokenPlain {
					foundPlain := false
					for _, tok := range tokens {
						if tok.Kind == CodeTokenPlain && strings.Contains(tok.Value, val) {
							foundPlain = true
							break
						}
					}
					if !foundPlain {
						t.Fatalf("expected plain span containing %q (tokens=%+v)", val, tokens)
					}
					continue
				}
				if kind, ok := got[val]; !ok || kind != wantKind {
					t.Fatalf("token %q: got kind=%v ok=%v, want %v (tokens=%+v)", val, kind, ok, wantKind, tokens)
				}
			}
		})
	}
}
