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
	if !run.HideText || run.Paint == nil || run.Advance < tokenChipMinWidth || run.Start != 0 || run.End != 25 || run.ChipLabel != "wox-plugin-creator" {
		t.Fatalf("chip run = %#v, want a painted replacement for the placeholder", run)
	}
}

func TestTokenChipHoverRevealsCloseAffordance(t *testing.T) {
	theme := Theme{ResultTitle: woxui.Color{A: 255}, ResultSubtitle: woxui.Color{A: 200}, ErrorText: woxui.Color{R: 200, A: 255}}
	run := NewTokenChipRun(0, 20, "query", nil, theme).WithDismissible()
	if !run.Dismissible {
		t.Fatal("query variable chips should be dismissible")
	}
	if got := withDismissibleChipHover([]TextFieldRichRun{run}, 0, theme, 0); len(got) != 1 || got[0].Advance != run.Advance {
		t.Fatalf("idle hover progress should keep the chip width, got %#v", got)
	}
	mid := withDismissibleChipHover([]TextFieldRichRun{run}, 0, theme, 0.5)
	if len(mid) != 1 || mid[0].Advance != run.Advance+tokenChipCloseSlot*0.5 {
		t.Fatalf("mid hover advance = %#v, want a half close slot", mid)
	}
	hovered := withDismissibleChipHover([]TextFieldRichRun{run}, 0, theme, 1)
	if len(hovered) != 1 || hovered[0].Advance != run.Advance+tokenChipCloseSlot {
		t.Fatalf("hovered chip advance = %#v, want %+v extra close slot", hovered, tokenChipCloseSlot)
	}
	editable := run.WithChipEdit()
	edited := withDismissibleChipHover([]TextFieldRichRun{editable}, 0, theme, 1)
	if len(edited) != 1 || edited[0].Advance != editable.Advance+tokenChipCloseSlot+tokenChipEditSlot {
		t.Fatalf("editable hovered chip advance = %#v, want edit and close slots", edited)
	}
	displayList := &woxui.DisplayList{}
	hovered[0].Paint(displayList, woxui.Rect{Width: hovered[0].Advance, Height: tokenChipHeight})
	if displayList.ImageDrawCount() == 0 {
		t.Fatal("hovered chip should paint the shared close icon")
	}
	idle := &woxui.DisplayList{}
	run.Paint(idle, woxui.Rect{Width: run.Advance, Height: tokenChipHeight})
	if idle.ImageDrawCount() != 0 {
		t.Fatal("idle chip should hide the close icon")
	}
	if got := tokenChipCloseColor(theme); got != theme.ErrorText {
		t.Fatalf("close color = %#v, want the theme danger color", got)
	}
}

func TestTextFieldDismissibleHitFindsCloseRegion(t *testing.T) {
	theme := Theme{ResultTitle: woxui.Color{A: 255}, ResultSubtitle: woxui.Color{A: 200}}
	value := "{wox:parameter?name=query}"
	end := len([]rune(value))
	run := NewTokenChipRun(0, end, "query", nil, theme).WithDismissible()
	hovered := withDismissibleChipHover([]TextFieldRichRun{run}, 0, theme, 1)
	state := woxui.TextEditingState{Text: value}
	style := woxui.TextStyle{Size: 13}
	start, stop, action, ok := textFieldDismissibleHit(state, nil, style, hovered, 1, textFieldLineHeight, 0, 400, false, false, woxui.Point{X: hovered[0].Advance - 2, Y: 10}, 1, 0)
	if !ok || action != tokenChipActionClose || start != 0 || stop != end {
		t.Fatalf("close hit = start %d end %d action %d ok %v", start, stop, action, ok)
	}
	_, _, action, ok = textFieldDismissibleHit(state, nil, style, hovered, 1, textFieldLineHeight, 0, 400, false, false, woxui.Point{X: 4, Y: 10}, 1, 0)
	if !ok || action != tokenChipActionBody {
		t.Fatalf("chip body hit = action %d ok %v", action, ok)
	}
	_, _, action, ok = textFieldDismissibleHit(state, nil, style, []TextFieldRichRun{run}, 1, textFieldLineHeight, 0, 400, false, false, woxui.Point{X: run.Advance - 2, Y: 10}, 0, 0)
	if !ok || action != tokenChipActionBody {
		t.Fatalf("unhovered chip should not treat its trailing edge as close, action %d ok %v", action, ok)
	}
	mid := withDismissibleChipHover([]TextFieldRichRun{run}, 0, theme, 0.4)
	_, _, action, ok = textFieldDismissibleHit(state, nil, style, mid, 1, textFieldLineHeight, 0, 400, false, false, woxui.Point{X: mid[0].Advance - 2, Y: 10}, 0.4, 0)
	if !ok || action != tokenChipActionBody {
		t.Fatalf("a still-opening close control should not dismiss, action %d ok %v", action, ok)
	}
	editable := withDismissibleChipHover([]TextFieldRichRun{run.WithChipEdit()}, 0, theme, 1)
	_, _, action, ok = textFieldDismissibleHit(state, nil, style, editable, 1, textFieldLineHeight, 0, 400, false, false, woxui.Point{X: editable[0].Advance - tokenChipCloseSlot - 2, Y: 10}, 1, 0)
	if !ok || action != tokenChipActionEdit {
		t.Fatalf("edit hit = action %d ok %v", action, ok)
	}
}
