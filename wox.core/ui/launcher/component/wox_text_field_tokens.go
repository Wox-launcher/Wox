package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TextFieldTokenRange is one inclusive-start exclusive-end rune span in a field value.
type TextFieldTokenRange struct {
	Start int
	End   int
}

func textFieldAtomicTokenBefore(tokens []TextFieldTokenRange, caret int) (TextFieldTokenRange, bool) {
	for _, token := range tokens {
		if token.End == caret {
			return token, true
		}
	}
	return TextFieldTokenRange{}, false
}

func textFieldAtomicTokenAfter(tokens []TextFieldTokenRange, caret int) (TextFieldTokenRange, bool) {
	for _, token := range tokens {
		if token.Start == caret {
			return token, true
		}
	}
	return TextFieldTokenRange{}, false
}

func textFieldAtomicTokenContaining(tokens []TextFieldTokenRange, caret int) (TextFieldTokenRange, bool) {
	for _, token := range tokens {
		if token.Start < caret && caret < token.End {
			return token, true
		}
	}
	return TextFieldTokenRange{}, false
}

// snapTextFieldAtomicCaret moves a caret that landed inside a token to the nearer edge.
func snapTextFieldAtomicCaret(tokens []TextFieldTokenRange, caret int) (int, bool) {
	token, ok := textFieldAtomicTokenContaining(tokens, caret)
	if !ok {
		return caret, false
	}
	if caret-token.Start <= token.End-caret {
		return token.Start, true
	}
	return token.End, true
}

// expandTextFieldAtomicSelection grows a range so it never splits an atomic token.
func expandTextFieldAtomicSelection(tokens []TextFieldTokenRange, start, end int) (int, int, bool) {
	if start > end {
		start, end = end, start
	}
	nextStart, nextEnd := start, end
	changed := false
	for _, token := range tokens {
		if start < token.End && end > token.Start {
			if token.Start < nextStart {
				nextStart = token.Start
				changed = true
			}
			if token.End > nextEnd {
				nextEnd = token.End
				changed = true
			}
		}
	}
	return nextStart, nextEnd, changed
}

func applyExpandedTextFieldAtomicSelection(controller *woxwidget.TextEditingController, tokens []TextFieldTokenRange, anchor, focus int) {
	if controller == nil || len(tokens) == 0 {
		return
	}
	start, end, changed := expandTextFieldAtomicSelection(tokens, anchor, focus)
	if !changed {
		return
	}
	if anchor <= focus {
		controller.SetSelection(start, end)
		return
	}
	controller.SetSelection(end, start)
}

func applyTextFieldAtomicSelection(controller *woxwidget.TextEditingController, anchor, focus int) {
	if controller == nil {
		return
	}
	if anchor == focus {
		controller.SetCaret(focus)
		return
	}
	controller.SetSelection(anchor, focus)
}

// handleTextFieldAtomicTokenKey deletes and navigates complete tokens as one unit.
func handleTextFieldAtomicTokenKey(controller *woxwidget.TextEditingController, tokens []TextFieldTokenRange, event woxui.KeyEvent) (bool, bool) {
	if controller == nil || len(tokens) == 0 || !event.Down || event.Composing || event.Modifiers.HasPrimary() {
		return false, false
	}
	current := controller.State()
	selection := current.Selection
	extend := event.Modifiers&woxui.KeyModifierShift != 0
	switch event.Key {
	case woxui.KeyBackspace:
		if !selection.Collapsed() {
			start, end, _ := expandTextFieldAtomicSelection(tokens, selection.Start(), selection.End())
			if selection.Anchor <= selection.Focus {
				applyTextFieldAtomicSelection(controller, start, end)
			} else {
				applyTextFieldAtomicSelection(controller, end, start)
			}
			return true, controller.DeleteSelection()
		}
		if token, ok := textFieldAtomicTokenBefore(tokens, selection.Focus); ok {
			applyTextFieldAtomicSelection(controller, token.Start, token.End)
			return true, controller.DeleteSelection()
		}
		if token, ok := textFieldAtomicTokenContaining(tokens, selection.Focus); ok {
			applyTextFieldAtomicSelection(controller, token.Start, token.End)
			return true, controller.DeleteSelection()
		}
	case woxui.KeyDelete:
		if !selection.Collapsed() {
			start, end, _ := expandTextFieldAtomicSelection(tokens, selection.Start(), selection.End())
			if selection.Anchor <= selection.Focus {
				applyTextFieldAtomicSelection(controller, start, end)
			} else {
				applyTextFieldAtomicSelection(controller, end, start)
			}
			return true, controller.DeleteSelection()
		}
		if token, ok := textFieldAtomicTokenAfter(tokens, selection.Focus); ok {
			applyTextFieldAtomicSelection(controller, token.Start, token.End)
			return true, controller.DeleteSelection()
		}
		if token, ok := textFieldAtomicTokenContaining(tokens, selection.Focus); ok {
			applyTextFieldAtomicSelection(controller, token.Start, token.End)
			return true, controller.DeleteSelection()
		}
	case woxui.KeyArrowLeft:
		if token, ok := textFieldAtomicTokenBefore(tokens, selection.Focus); ok {
			if extend {
				applyTextFieldAtomicSelection(controller, selection.Anchor, token.Start)
			} else {
				controller.SetCaret(token.Start)
			}
			return true, false
		}
		if token, ok := textFieldAtomicTokenContaining(tokens, selection.Focus); ok {
			if extend {
				applyTextFieldAtomicSelection(controller, selection.Anchor, token.Start)
			} else {
				controller.SetCaret(token.Start)
			}
			return true, false
		}
	case woxui.KeyArrowRight:
		if token, ok := textFieldAtomicTokenAfter(tokens, selection.Focus); ok {
			if extend {
				applyTextFieldAtomicSelection(controller, selection.Anchor, token.End)
			} else {
				controller.SetCaret(token.End)
			}
			return true, false
		}
		if token, ok := textFieldAtomicTokenContaining(tokens, selection.Focus); ok {
			if extend {
				applyTextFieldAtomicSelection(controller, selection.Anchor, token.End)
			} else {
				controller.SetCaret(token.End)
			}
			return true, false
		}
	}
	return false, false
}
