package preview

import (
	"testing"

	"wox/plugin/system/shell/interactive"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

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
		return ShellPreviewView(ShellPreviewProps{
			Width: 240, Height: 180, CellWidth: 8, CellHeight: 16,
			Frame:  &interactive.Frame{Size: interactive.Size{Columns: 30, Rows: 10}},
			Output: &ShellOutputProps{SessionID: "s", Text: "hello output", OnCopy: func() {}, OnSelectAll: func() {}},
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
