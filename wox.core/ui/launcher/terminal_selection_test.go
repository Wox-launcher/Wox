package launcher

import (
	"runtime"
	"testing"

	woxui "wox/ui/runtime"
)

func TestTerminalSelectedTextUsesByteRange(t *testing.T) {
	state := &terminalPreviewState{Text: "hello 世界", SelectionAnchor: 6, SelectionFocus: len("hello 世界")}
	if got := terminalSelectedText(state); got != "世界" {
		t.Fatalf("selected = %q, want 世界", got)
	}
}

func TestRemapTerminalSelectionFollowsAbsoluteCursors(t *testing.T) {
	state := &terminalPreviewState{Text: "hello", BaseCursor: 10, SelectionAnchor: 1, SelectionFocus: 4}
	state.Text = "prehello"
	state.BaseCursor = 7
	remapTerminalSelection(state, 10)
	if terminalSelectedText(state) != "ell" {
		t.Fatalf("prepended selection = %q [%d:%d], want ell", terminalSelectedText(state), state.SelectionAnchor, state.SelectionFocus)
	}

	state.Text = "ell"
	state.BaseCursor = 11
	remapTerminalSelection(state, 7)
	if terminalSelectedText(state) != "ell" {
		t.Fatalf("trimmed selection = %q, want ell", terminalSelectedText(state))
	}

	state.Text = "zzz"
	state.BaseCursor = 40
	remapTerminalSelection(state, 11)
	if state.SelectionAnchor != state.SelectionFocus {
		t.Fatalf("cleared selection = %d:%d", state.SelectionAnchor, state.SelectionFocus)
	}
}

func TestApplyTerminalChunkKeepsSelectionOnAppend(t *testing.T) {
	app := &App{terminalPreview: &terminalPreviewState{SessionID: "s", Text: "hello", BaseCursor: 0, SelectionAnchor: 1, SelectionFocus: 4}}
	app.applyTerminalChunk(terminalChunk{SessionID: "s", CursorStart: 5, CursorEnd: 8, Content: "!!!"})
	if app.terminalPreview.Text != "hello!!!" || terminalSelectedText(app.terminalPreview) != "ell" {
		t.Fatalf("appended text = %q selected = %q", app.terminalPreview.Text, terminalSelectedText(app.terminalPreview))
	}
}

// TestTerminalReverseSelection preserves the anchor across output changes and continued dragging.
func TestTerminalReverseSelection(t *testing.T) {
	app := &App{terminalPreview: &terminalPreviewState{SessionID: "s", Text: "abcdef", SelectionAnchor: 5, SelectionFocus: 2}}
	app.applyTerminalChunk(terminalChunk{SessionID: "s", CursorStart: 6, CursorEnd: 7, Content: "!"})
	state := app.terminalPreview
	if state.SelectionAnchor != 5 || state.SelectionFocus != 2 {
		t.Fatalf("appended selection = %d:%d, want 5:2", state.SelectionAnchor, state.SelectionFocus)
	}
	app.extendTerminalSelection(1)
	if got := terminalSelectedText(state); got != "bcde" {
		t.Fatalf("extended selection = %q, want bcde", got)
	}
	state.Text, state.BaseCursor = "cdef!", 2
	remapTerminalSelection(state, 0)
	if state.SelectionAnchor != 3 || state.SelectionFocus != 0 || terminalSelectedText(state) != "cde" {
		t.Fatalf("trimmed selection = %d:%d (%q), want 3:0 (cde)", state.SelectionAnchor, state.SelectionFocus, terminalSelectedText(state))
	}
	state.Text, state.BaseCursor = "abcdef!", 0
	remapTerminalSelection(state, 2)
	if state.SelectionAnchor != 5 || state.SelectionFocus != 2 {
		t.Fatalf("prepended selection = %d:%d, want 5:2", state.SelectionAnchor, state.SelectionFocus)
	}
}

func TestTerminalPreviewCopyShortcutConsumesSelection(t *testing.T) {
	app := &App{terminalPreview: &terminalPreviewState{Text: "hello world", SelectionAnchor: 0, SelectionFocus: 5}}
	modifiers := woxui.KeyModifierControl
	if runtime.GOOS == "darwin" {
		modifiers = woxui.KeyModifierMeta
	}
	if !app.onTerminalPreviewKey(woxui.KeyEvent{Key: woxui.Key("c"), Modifiers: modifiers, Down: true}) {
		t.Fatal("ctrl/cmd+c should copy a terminal highlight instead of the query")
	}
	app.terminalPreview.SelectionAnchor, app.terminalPreview.SelectionFocus = 0, 0
	if app.onTerminalPreviewKey(woxui.KeyEvent{Key: woxui.Key("c"), Modifiers: modifiers, Down: true}) {
		t.Fatal("collapsed terminal selection must leave copy for the query editor")
	}
}

func TestSelectAllTerminalPreviewHighlightsLoadedOutput(t *testing.T) {
	app := &App{terminalPreview: &terminalPreviewState{Text: "abc"}}
	app.selectAllTerminalPreview()
	if terminalSelectedText(app.terminalPreview) != "abc" {
		t.Fatalf("select all = %q", terminalSelectedText(app.terminalPreview))
	}
}

func TestClearTerminalSelectionDropsHighlight(t *testing.T) {
	app := &App{terminalPreview: &terminalPreviewState{Text: "abc", SelectionAnchor: 0, SelectionFocus: 3}}
	app.clearTerminalSelection()
	if terminalSelectedText(app.terminalPreview) != "" {
		t.Fatalf("cleared selection = %q", terminalSelectedText(app.terminalPreview))
	}
}
