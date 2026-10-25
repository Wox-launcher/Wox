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

func (a *queryController) completionHintValidLocked(hint *queryCompletionHint) bool {
	if hint == nil || !a.completionEnabled() || a.query.QueryType != "input" || hint.InputPrefix != a.editor.State().Text || hint.Suffix == "" {
		return false
	}
	state := a.editor.State()
	return state.Composition == "" && state.Selection.Collapsed() && state.Selection.Focus == len([]rune(state.Text)) && strings.HasPrefix(hint.CompletionText, state.Text)
}

// reuseCompletionHintLocked avoids blank frames while the next candidate is computed.
func (a *queryController) reuseCompletionHintLocked(text string) {
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

func (a *queryController) acceptQueryCompletionHint() bool {
	if !a.completionHintValidLocked(a.completionHint) {
		return false
	}
	hint := *a.completionHint
	a.rememberQueryHint()
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
	_ = a.window().Invalidate()
	return true
}

// autoCompleteQueryFromSelectedResult mirrors Flutter's Shift+Tab title completion.
func (a *queryController) autoCompleteQueryFromSelectedResult() {
	if a.resultList.selected < 0 || a.resultList.selected >= len(a.resultList.results) || a.resultList.results[a.resultList.selected].IsGroup {
		return
	}
	title := a.resultList.results[a.resultList.selected].Title
	if title == "" {
		return
	}
	a.rememberQueryHint()
	a.editor.SetText(title, false)
	a.applyQueryTextChangeLocked(title)
	a.reconcileSelectedPreview()
	_ = a.window().Invalidate()
	if err := a.sendCurrentQuery(); err != nil {
		log.Printf("send selected result completion: %v", err)
	}
}
