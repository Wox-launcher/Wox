package ui

import (
	"context"
	"errors"
	"fmt"

	"wox/plugin"
	"wox/plugin/system/shell/terminal"
	"wox/setting"
	"wox/ui/contract"
	"wox/util"
)

// CoreServices implements the typed services consumed by the embedded UI.
type CoreServices struct{}

// NewCoreServices creates the process-local UI service facade.
func NewCoreServices() *CoreServices {
	return &CoreServices{}
}

// AttachView binds the embedded launcher to the core-owned UI manager.
func (s *CoreServices) AttachView(view contract.View) {
	GetUIManager().AttachView(view)
}

// Ready applies core startup behavior after the native window is initialized.
func (s *CoreServices) Ready(ctx context.Context, sessionID string) error {
	ctx = uiServiceContext(ctx, sessionID)
	GetUIManager().PostUIReady(ctx)
	startCloudSyncManagerAfterUIReady(ctx)
	return nil
}

// RegisterInstance makes a secondary launcher addressable by its UI session.
func (s *CoreServices) RegisterInstance(_ context.Context, view contract.View) error {
	if view == nil || view.SessionID() == "" {
		return errors.New("secondary UI view and session are required")
	}
	GetUIManager().RegisterView(view)
	return nil
}

// DestroyInstance releases session-routed view and plugin query state.
func (s *CoreServices) DestroyInstance(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	GetUIManager().UnregisterView(sessionID)
	GetUIManager().PostOnInstanceDestroyed(uiServiceContext(ctx, sessionID))
	return nil
}

// Shown records that the launcher window became visible.
func (s *CoreServices) Shown(ctx context.Context, sessionID string) error {
	GetUIManager().PostOnShow(uiServiceContext(ctx, sessionID))
	return nil
}

// Hidden records that the final user-facing Wox window became hidden.
func (s *CoreServices) Hidden(ctx context.Context, sessionID string) error {
	GetUIManager().PostOnHide(uiServiceContext(ctx, sessionID))
	return nil
}

// FocusLost applies the core setting that controls focus-loss dismissal.
func (s *CoreServices) FocusLost(ctx context.Context, sessionID string) error {
	ctx = uiServiceContext(ctx, sessionID)
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if woxSetting.HideOnLostFocus.Get() {
		GetUIManager().GetUI(ctx).HideApp(ctx)
	}
	return nil
}

// SettingViewChanged keeps core management state synchronized with the independent settings window.
func (s *CoreServices) SettingViewChanged(ctx context.Context, sessionID string, inSettingView bool) error {
	GetUIManager().PostOnSetting(uiServiceContext(ctx, sessionID), inSettingView)
	return nil
}

// StartQuery schedules the core query pipeline and streams typed snapshots to the view.
func (s *CoreServices) StartQuery(ctx context.Context, request contract.QueryRequest, view contract.QueryView) error {
	if view == nil {
		return errors.New("query view is required")
	}
	if request.SessionID == "" || request.Query.QueryId == "" {
		return errors.New("query session and query id are required")
	}
	ctx = util.WithQueryIdContext(uiServiceContext(ctx, request.SessionID), request.Query.QueryId)
	util.Go(ctx, "handle typed UI query", func() {
		runUIQuery(ctx, request, view)
	})
	return nil
}

// QueryMRU restores typed results for the current launcher session.
func (s *CoreServices) QueryMRU(ctx context.Context, sessionID string, queryID string) ([]plugin.QueryResultUI, error) {
	ctx = util.WithQueryIdContext(uiServiceContext(ctx, sessionID), queryID)
	results := plugin.GetPluginManager().QueryMRU(ctx, sessionID, queryID)
	logger.Info(ctx, fmt.Sprintf("found %d MRU results from UI", len(results)))
	return results, nil
}

// ExecuteAction schedules one cached query-result action.
func (s *CoreServices) ExecuteAction(ctx context.Context, sessionID string, queryID string, resultID string, actionID string) error {
	ctx = util.WithQueryIdContext(uiServiceContext(ctx, sessionID), queryID)
	util.Go(ctx, "execute typed UI action", func() {
		if err := plugin.GetPluginManager().ExecuteAction(ctx, sessionID, queryID, resultID, actionID); err != nil {
			logger.Error(ctx, fmt.Sprintf("failed to execute UI action: %v", err))
		}
	})
	return nil
}

// SubmitFormAction schedules one cached form action with already-decoded values.
func (s *CoreServices) SubmitFormAction(ctx context.Context, sessionID string, queryID string, resultID string, actionID string, values map[string]string) error {
	ctx = util.WithQueryIdContext(uiServiceContext(ctx, sessionID), queryID)
	util.Go(ctx, "submit typed UI form action", func() {
		if err := plugin.GetPluginManager().SubmitFormAction(ctx, sessionID, queryID, resultID, actionID, values); err != nil {
			logger.Error(ctx, fmt.Sprintf("failed to submit UI form action: %v", err))
		}
	})
	return nil
}

// AcceptQueryCompletionHint records accepted inline-completion feedback.
func (s *CoreServices) AcceptQueryCompletionHint(ctx context.Context, sessionID string, inputPrefix string, completionText string, source string) error {
	ctx = uiServiceContext(ctx, sessionID)
	if !setting.GetSettingManager().RecordQueryCompletionFeedback(ctx, inputPrefix, completionText, source) {
		logger.Debug(ctx, fmt.Sprintf("ignore invalid query completion feedback: inputPrefix=%q, completionText=%q, source=%q", inputPrefix, completionText, source))
	}
	return nil
}

// ExecuteToolbarMessageAction invokes one action from the session-owned toolbar snapshot.
func (s *CoreServices) ExecuteToolbarMessageAction(ctx context.Context, sessionID string, toolbarMessageID string, actionID string) error {
	if toolbarMessageID == "" || actionID == "" {
		return errors.New("toolbar message and action ids are required")
	}
	return plugin.GetPluginManager().ExecuteToolbarMsgAction(uiServiceContext(ctx, sessionID), sessionID, toolbarMessageID, actionID)
}

// SubscribeTerminal starts or rewinds the output stream for one launcher session.
func (s *CoreServices) SubscribeTerminal(ctx context.Context, uiSessionID string, terminalSessionID string, cursor int64) (terminal.SessionState, error) {
	if uiSessionID == "" || terminalSessionID == "" {
		return terminal.SessionState{}, errors.New("UI and terminal session ids are required")
	}
	return terminal.GetSessionManager().Subscribe(uiServiceContext(ctx, uiSessionID), uiSessionID, terminalSessionID, cursor)
}

// UnsubscribeTerminal releases one launcher session from terminal output updates.
func (s *CoreServices) UnsubscribeTerminal(_ context.Context, uiSessionID string, terminalSessionID string) error {
	if uiSessionID == "" || terminalSessionID == "" {
		return errors.New("UI and terminal session ids are required")
	}
	terminal.GetSessionManager().Unsubscribe(uiSessionID, terminalSessionID)
	return nil
}

func uiServiceContext(ctx context.Context, sessionID string) context.Context {
	if ctx == nil || util.GetContextTraceId(ctx) == "" {
		ctx = util.NewTraceContext()
	}
	return util.WithSessionContext(ctx, sessionID)
}
