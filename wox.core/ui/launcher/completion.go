package launcher

import (
	"context"
	"log"
	"strings"
	"unicode/utf8"
)

type queryCompletionHint struct {
	InputPrefix            string `json:"InputPrefix"`
	CompletionText         string `json:"CompletionText"`
	Suffix                 string `json:"Suffix"`
	Source                 string `json:"Source"`
	Score                  int    `json:"Score"`
	DeletionReuseMinLength int    `json:"DeletionReuseMinLength"`
}

func (a *App) completionHintValidLocked(hint *queryCompletionHint) bool {
	if hint == nil || !a.generalSettings.Data().EnableQueryCompletionHint || a.query.QueryType != "input" || hint.InputPrefix != a.editor.State().Text || hint.Suffix == "" {
		return false
	}
	state := a.editor.State()
	return state.Composition == "" && state.Selection.Collapsed() && state.Selection.Focus == len([]rune(state.Text)) && strings.HasPrefix(hint.CompletionText, state.Text)
}

// reuseCompletionHintLocked avoids blank frames while the next candidate is computed.
func (a *App) reuseCompletionHintLocked(text string) {
	if a.completionHint == nil {
		return
	}
	hint := a.completionHint
	appending := len(text) > len(hint.InputPrefix) && strings.HasPrefix(text, hint.InputPrefix)
	// Only backend-approved histories can survive deletion; plugin command prefixes
	// may become ambiguous and must wait for a fresh candidate.
	deleting := len(text) < len(hint.InputPrefix) && strings.HasPrefix(hint.InputPrefix, text) && hint.DeletionReuseMinLength > 0 && utf8.RuneCountInString(strings.TrimSpace(text)) >= hint.DeletionReuseMinLength
	if (!appending && !deleting) || !strings.HasPrefix(hint.CompletionText, text) {
		a.completionHint = nil
		return
	}
	suffix := strings.TrimPrefix(a.completionHint.CompletionText, text)
	if suffix == "" {
		a.completionHint = nil
		return
	}
	a.completionHint.InputPrefix = text
	a.completionHint.Suffix = suffix
}

func (a *App) acceptQueryCompletionHint() bool {
	if !a.completionHintValidLocked(a.completionHint) {
		return false
	}
	hint := *a.completionHint
	a.editor.SetText(hint.CompletionText, false)
	a.applyQueryTextChangeLocked(hint.CompletionText)
	a.completionHint = nil
	a.reconcileSelectedPreview()
	if err := a.services.AcceptQueryCompletionHint(context.Background(), a.sessionID, hint.InputPrefix, hint.CompletionText, hint.Source); err != nil {
		log.Printf("record accepted query completion hint: %v", err)
	}
	if err := a.sendCurrentQuery(); err != nil {
		log.Printf("send accepted query completion: %v", err)
	}
	_ = a.window.Invalidate()
	return true
}

// autoCompleteQueryFromSelectedResult mirrors Flutter's Shift+Tab title completion.
func (a *App) autoCompleteQueryFromSelectedResult() {
	if a.selected < 0 || a.selected >= len(a.results) || a.results[a.selected].IsGroup {
		return
	}
	title := a.results[a.selected].Title
	if title == "" {
		return
	}
	a.editor.SetText(title, false)
	a.applyQueryTextChangeLocked(title)
	a.reconcileSelectedPreview()
	_ = a.window.Invalidate()
	if err := a.sendCurrentQuery(); err != nil {
		log.Printf("send selected result completion: %v", err)
	}
}
