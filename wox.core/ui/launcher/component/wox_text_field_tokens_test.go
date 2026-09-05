package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestHandleTextFieldAtomicTokenKeyDeletesWholePlaceholder(t *testing.T) {
	tag := "{skill:wox-plugin-creator}"
	start := len([]rune("hi "))
	end := start + len([]rune(tag))
	tokens := []TextFieldTokenRange{{Start: start, End: end}}
	controller := woxwidget.NewTextEditingController("hi " + tag + " x")
	controller.SetCaret(end)

	handled, changed := handleTextFieldAtomicTokenKey(controller, tokens, woxui.KeyEvent{Key: woxui.KeyBackspace, Down: true})
	if !handled || !changed {
		t.Fatal("backspace after a skill tag should delete the whole token")
	}
	if got := controller.Text(); got != "hi  x" {
		t.Fatalf("backspace text = %q, want the whole tag removed", got)
	}

	controller.SetText("hi "+tag+" x", false)
	controller.SetCaret(start)
	handled, changed = handleTextFieldAtomicTokenKey(controller, tokens, woxui.KeyEvent{Key: woxui.KeyDelete, Down: true})
	if !handled || !changed {
		t.Fatal("delete before a skill tag should delete the whole token")
	}
	if got := controller.Text(); got != "hi  x" {
		t.Fatalf("delete text = %q, want the whole tag removed", got)
	}
}

func TestHandleTextFieldAtomicTokenKeyJumpsPlaceholder(t *testing.T) {
	tag := "{skill:wox-plugin-creator}"
	end := len([]rune(tag))
	tokens := []TextFieldTokenRange{{Start: 0, End: end}}
	controller := woxwidget.NewTextEditingController(tag)
	controller.SetCaret(end)

	handled, changed := handleTextFieldAtomicTokenKey(controller, tokens, woxui.KeyEvent{Key: woxui.KeyArrowLeft, Down: true})
	if !handled || changed {
		t.Fatal("left arrow should jump to the start of the token")
	}
	if got := controller.State().Selection.Focus; got != 0 {
		t.Fatalf("caret after left = %d, want 0", got)
	}
	handled, changed = handleTextFieldAtomicTokenKey(controller, tokens, woxui.KeyEvent{Key: woxui.KeyArrowRight, Down: true})
	if !handled || changed {
		t.Fatal("right arrow should jump to the end of the token")
	}
	if got := controller.State().Selection.Focus; got != end {
		t.Fatalf("caret after right = %d, want token end", got)
	}
}

func TestSnapTextFieldAtomicCaretMovesToNearerEdge(t *testing.T) {
	tokens := []TextFieldTokenRange{{Start: 1, End: 20}}
	if got, ok := snapTextFieldAtomicCaret(tokens, 3); !ok || got != 1 {
		t.Fatalf("snap near start = %d %v, want 1", got, ok)
	}
	if got, ok := snapTextFieldAtomicCaret(tokens, 18); !ok || got != 20 {
		t.Fatalf("snap near end = %d %v, want 20", got, ok)
	}
	if _, ok := snapTextFieldAtomicCaret(tokens, 1); ok {
		t.Fatal("caret on the token edge should stay put")
	}
}

func TestNewTokenChipRunHidesPlaceholderText(t *testing.T) {
	run := NewTokenChipRun(0, 25, "wox-plugin-creator", nil, Theme{ResultTitle: woxui.Color{A: 255}, ResultSubtitle: woxui.Color{A: 200}})
	if !run.HideText || run.Paint == nil || run.Advance < tokenChipMinWidth || run.Start != 0 || run.End != 25 {
		t.Fatalf("chip run = %#v, want a painted replacement for the placeholder", run)
	}
}
