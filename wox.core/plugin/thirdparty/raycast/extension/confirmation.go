package extension

import (
	"context"
	"encoding/json"

	"wox/common/icons"
	"wox/plugin"
)

// resultConfirmation belongs to one invocation and cannot be accepted by a stale result callback.
type resultConfirmation struct {
	resultID string
	answer   chan bool
}

// confirmAlert temporarily replaces the invoking row's actions without adding UI or query scope.
func (p *extensionPlugin) confirmAlert(ctx context.Context, s *commandSession, raw json.RawMessage) (bool, error) {
	var options struct {
		Title        string `json:"title"`
		Message      string `json:"message"`
		PrimaryTitle string `json:"primaryTitle"`
		DismissTitle string `json:"dismissTitle"`
	}
	if err := json.Unmarshal(raw, &options); err != nil {
		return false, err
	}
	s.displayMu.Lock()
	s.mu.Lock()
	if s.closed || s.actionResultID == "" || s.confirmation != nil {
		s.mu.Unlock()
		s.displayMu.Unlock()
		return false, nil
	}
	pending := &resultConfirmation{resultID: s.actionResultID, answer: make(chan bool, 1)}
	s.confirmation = pending
	s.signalConfirmationLocked()
	s.mu.Unlock()
	if options.PrimaryTitle == "" {
		options.PrimaryTitle = "i18n:plugin_raycast_confirm"
	}
	if options.DismissTitle == "" {
		options.DismissTitle = "i18n:ui_cancel"
	}
	actions := []plugin.QueryResultAction{
		{Id: "raycast:confirm", Name: options.PrimaryTitle, Icon: icons.Get(icons.ActionExecute), IsDefault: true, Hotkey: "enter", PreventHideAfterAction: true,
			Action: func(context.Context, plugin.ActionContext) { p.resolveConfirmation(s, pending, true) }},
		{Id: "raycast:cancel", Name: options.DismissTitle, Hotkey: "ctrl+enter", PreventHideAfterAction: true, ShowInToolbar: true,
			Action: func(context.Context, plugin.ActionContext) { p.resolveConfirmation(s, pending, false) }},
	}
	shown := p.api.UpdateResult(ctx, plugin.UpdatableResult{Id: pending.resultID, Title: &options.Title, SubTitle: &options.Message, Actions: &actions})
	s.displayMu.Unlock()
	if !shown {
		p.resolveConfirmation(s, pending, false)
	}
	select {
	case answer := <-pending.answer:
		return answer, nil
	case <-s.done:
		p.resolveConfirmation(s, pending, false)
		return false, nil
	case <-ctx.Done():
		p.resolveConfirmation(s, pending, false)
		return false, nil
	}
}

// signalConfirmationLocked pauses/resumes the action timeout while a user decision is outstanding.
func (s *commandSession) signalConfirmationLocked() {
	select {
	case s.confirmationChanged <- struct{}{}:
	default:
	}
}

// resolveConfirmation restores the latest React row before allowing the suspended action to continue.
func (p *extensionPlugin) resolveConfirmation(s *commandSession, pending *resultConfirmation, accepted bool) {
	s.displayMu.Lock()
	defer s.displayMu.Unlock()
	s.mu.Lock()
	if s.confirmation != pending {
		s.mu.Unlock()
		return
	}
	s.confirmation = nil
	closed := s.closed
	response, ctx := s.response, s.ctx
	s.signalConfirmationLocked()
	s.scheduleLocked()
	s.mu.Unlock()
	if !closed {
		for _, row := range p.bind(s, response).Results {
			if row.Id == pending.resultID {
				p.api.UpdateResult(ctx, plugin.UpdatableResult{Id: row.Id, Title: &row.Title, SubTitle: &row.SubTitle, Actions: &row.Actions})
				break
			}
		}
	}
	pending.answer <- accepted && !closed
}

// cancelConfirmation rejects the pending decision before a query replaces its owning result.
func (p *extensionPlugin) cancelConfirmation(s *commandSession) {
	s.mu.Lock()
	pending := s.confirmation
	s.mu.Unlock()
	if pending != nil {
		p.resolveConfirmation(s, pending, false)
	}
}
