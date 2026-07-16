package contract

import (
	"context"

	"wox/plugin/system/shell/terminal"
)

// InteractionServices owns launcher actions that are not part of query execution.
type InteractionServices interface {
	ExecuteToolbarMessageAction(ctx context.Context, sessionID string, toolbarMessageID string, actionID string) error
	SubscribeTerminal(ctx context.Context, uiSessionID string, terminalSessionID string, cursor int64) (terminal.SessionState, error)
	UnsubscribeTerminal(ctx context.Context, uiSessionID string, terminalSessionID string) error
}
