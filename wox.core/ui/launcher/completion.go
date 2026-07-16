package launcher

import (
	"context"
	"encoding/json"
	"log"
	"strings"
)

type queryCompletionHint struct {
	InputPrefix    string `json:"InputPrefix"`
	CompletionText string `json:"CompletionText"`
	Suffix         string `json:"Suffix"`
	Source         string `json:"Source"`
	Score          int    `json:"Score"`
}

func (a *App) applyQueryCompletionHint(raw json.RawMessage) {
	var payload struct {
		QueryID        string               `json:"QueryId"`
		CompletionHint *queryCompletionHint `json:"CompletionHint"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		log.Printf("decode query completion hint: %v", err)
		return
	}
	a.mu.Lock()
	if payload.QueryID != a.query.QueryID || !a.completionHintValidLocked(payload.CompletionHint) {
		if payload.QueryID == a.query.QueryID {
			a.completionHint = nil
		}
		a.mu.Unlock()
		return
	}
	copy := *payload.CompletionHint
	a.completionHint = &copy
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) completionHintValidLocked(hint *queryCompletionHint) bool {
	if hint == nil || !a.settings.EnableQueryCompletionHint || a.query.QueryType != "input" || hint.InputPrefix != a.editor.State().Text || hint.Suffix == "" {
		return false
	}
	state := a.editor.State()
	return state.Composition == "" && state.Selection.Collapsed() && state.Selection.Focus == len([]rune(state.Text)) && strings.HasPrefix(hint.CompletionText, state.Text)
}

func (a *App) reuseCompletionHintLocked(text string) {
	if a.completionHint == nil || len([]rune(text)) <= len([]rune(a.completionHint.InputPrefix)) || !strings.HasPrefix(text, a.completionHint.InputPrefix) || !strings.HasPrefix(a.completionHint.CompletionText, text) {
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
	a.mu.Lock()
	if !a.completionHintValidLocked(a.completionHint) {
		a.mu.Unlock()
		return false
	}
	hint := *a.completionHint
	a.editor.SetText(hint.CompletionText, false)
	a.applyQueryTextChangeLocked(hint.CompletionText)
	a.completionHint = nil
	a.mu.Unlock()
	a.deactivateTerminalPreview()
	if err := a.services.AcceptQueryCompletionHint(context.Background(), a.sessionID, hint.InputPrefix, hint.CompletionText, hint.Source); err != nil {
		log.Printf("record accepted query completion hint: %v", err)
	}
	if err := a.sendCurrentQuery(); err != nil {
		log.Printf("send accepted query completion: %v", err)
	}
	_ = a.window.Invalidate()
	return true
}
