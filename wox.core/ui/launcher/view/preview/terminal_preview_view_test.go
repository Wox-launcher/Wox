package preview

import (
	"reflect"
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestTerminalHighlightSegments preserves byte offsets across wrapped lines and skips inactive search.
func TestTerminalHighlightSegments(t *testing.T) {
	value := "first\n中文 match\nlast"
	lines := []string{"first", "中文", "match", "last"}
	if got := terminalHighlightSegments(value, lines, nil); got != nil {
		t.Fatalf("inactive search prepared highlights: %#v", got)
	}
	got := terminalHighlightSegments(value, lines, []TerminalMatch{{Start: 6, End: 18}})
	want := []terminalHighlightSegment{{line: 1, start: 0, end: 6}, {line: 2, start: 0, end: 5}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("highlight segments = %#v, want %#v", got, want)
	}
}

func TestTerminalOffsetAtUsesWrappedLineOrigins(t *testing.T) {
	value := "first\n中文 match\nlast"
	layout := woxwidget.TextBlockLayout{Lines: []string{"first", "中文", "match", "last"}, LineHeight: 18}
	style := woxui.TextStyle{Size: 12}
	got := terminalOffsetAt(value, layout, nil, style, woxui.Point{X: 0, Y: 36}, 18)
	if value[got:got+5] != "match" {
		t.Fatalf("offset %d = %q, want match", got, value[got:])
	}
	end := terminalOffsetAt(value, layout, nil, style, woxui.Point{X: 0, Y: 90}, 18)
	if end != len(value) {
		t.Fatalf("below last line offset = %d, want %d", end, len(value))
	}
}

// TestTerminalOffsetAtPreservesBlankLines covers original byte offsets across newline formats and wrapping.
func TestTerminalOffsetAtPreservesBlankLines(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n", "\r"} {
		value := newline + "first" + newline + newline + "中文 match" + newline
		layout := woxwidget.TextBlockLayout{Lines: []string{"", "first", "", "中文", "match", ""}, LineHeight: 18}
		want := []int{0, len(newline), len("first") + 2*len(newline), len("first") + 3*len(newline), len("first中文 ") + 3*len(newline), len(value)}
		for line, offset := range want {
			got := terminalOffsetAt(value, layout, nil, woxui.TextStyle{Size: 12}, woxui.Point{Y: float32(line) * 18}, 18)
			if got != offset {
				t.Fatalf("newline %q line %d offset = %d, want %d", newline, line, got, offset)
			}
		}
	}
}

func TestTerminalWordAndLineByteRanges(t *testing.T) {
	value := "hello 世界\nnext"
	start, end := TerminalWordByteRange(value, 1)
	if value[start:end] != "hello" {
		t.Fatalf("word = %q, want hello", value[start:end])
	}
	start, end = TerminalWordByteRange(value, len("hello "))
	if value[start:end] != "世界" {
		t.Fatalf("cjk word = %q, want 世界", value[start:end])
	}
	start, end = TerminalLineByteRange(value, 1)
	if value[start:end] != "hello 世界" {
		t.Fatalf("line = %q, want hello 世界", value[start:end])
	}
}

func TestTerminalOutputExposesCopyActions(t *testing.T) {
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return TerminalPreviewView(TerminalPreviewProps{
			Width: 240, Height: 180, SessionID: "s", Text: "hello output",
			LayoutText: func(value string, style woxui.TextStyle, width, lineHeight float32) woxwidget.TextBlockLayout {
				return woxwidget.LayoutTextBlock(nil, value, style, width, 0, lineHeight)
			},
			OnSelectionStart: func(int, woxui.KeyModifiers) {},
			OnCopy:           func() {},
			OnSelectAll:      func() {},
		})
	})
	host.AttachServices(terminalPreviewHostServices{})
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 240, Height: 180}, PixelSize: woxui.PixelSize{Width: 240, Height: 180}, Scale: 1})
	var found bool
	for _, node := range host.Snapshot().Tree.Nodes {
		if node.AutomationID != "launcher.preview.terminal.output" {
			continue
		}
		found = true
		if !node.HasTextSelection || node.Value != "hello output" {
			t.Fatalf("output semantics = %#v", node)
		}
		if !containsAction(node.Actions, woxui.AccessibilityActionCopy) || !containsAction(node.Actions, woxui.AccessibilityActionSelectAll) {
			t.Fatalf("output actions = %v", node.Actions)
		}
	}
	if !found {
		t.Fatal("terminal output semantics missing")
	}
}

func containsAction(actions []woxui.AccessibilityAction, want woxui.AccessibilityAction) bool {
	for _, action := range actions {
		if action == want {
			return true
		}
	}
	return false
}

type terminalPreviewHostServices struct{}

func (terminalPreviewHostServices) MeasureText(text string, style woxui.TextStyle) (woxui.TextMetrics, error) {
	return woxui.TextMetrics{Size: woxui.Size{Width: float32(utf8RuneCount(text)) * max(style.Size/2, 1), Height: max(style.Size, 1)}}, nil
}

func (terminalPreviewHostServices) Invalidate() error { return nil }

func (terminalPreviewHostServices) InvalidateRect(woxui.Rect) error { return nil }

func (terminalPreviewHostServices) SetTextInputState(woxui.TextInputState) error { return nil }

func (terminalPreviewHostServices) SetPointerCursor(woxui.PointerCursor) error { return nil }

func (terminalPreviewHostServices) UpdateAccessibility(woxui.AccessibilityTree, woxui.AccessibilityActionHandler) error {
	return nil
}

func utf8RuneCount(value string) int {
	count := 0
	for range value {
		count++
	}
	return count
}
