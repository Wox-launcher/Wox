package contract

import (
	"context"

	"wox/common"
	"wox/plugin"
)

// Services is the typed boundary exposed by core to the embedded UI.
type Services interface {
	LifecycleServices
	QueryServices
	InteractionServices
}

// QueryRequest contains one already-decoded query from the launcher.
type QueryRequest struct {
	RequestID          string
	SessionID          string
	Query              common.PlainQuery
	SkipCompletionHint bool
	SentTimestamp      int64
}

// QueryResponse contains one incremental or terminal result snapshot.
type QueryResponse struct {
	QueryID  string
	Response plugin.QueryResponseUI
	IsFinal  bool
}

// QueryView receives typed query output from core.
type QueryView interface {
	ApplyQueryResponse(ctx context.Context, response QueryResponse)
	ApplyQueryCompletionHint(ctx context.Context, queryID string, hint *plugin.QueryCompletionHint)
	ApplyQueryError(ctx context.Context, queryID string, err error)
}

// QueryServices exposes high-frequency query and action behavior without transport encoding.
type QueryServices interface {
	StartQuery(ctx context.Context, request QueryRequest, view QueryView) error
	QueryMRU(ctx context.Context, sessionID string, queryID string) ([]plugin.QueryResultUI, error)
	ExecuteAction(ctx context.Context, sessionID string, queryID string, resultID string, actionID string) error
	SubmitFormAction(ctx context.Context, sessionID string, queryID string, resultID string, actionID string, values map[string]string) error
	AcceptQueryCompletionHint(ctx context.Context, sessionID string, inputPrefix string, completionText string, source string) error
}
