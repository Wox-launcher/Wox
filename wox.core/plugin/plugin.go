package plugin

import "context"

var AllSystemPlugin []SystemPlugin

type Plugin interface {
	Init(ctx context.Context, initParams InitParams)
	Query(ctx context.Context, query Query) QueryResponse
}

type SystemPlugin interface {
	Plugin
	GetMetadata() Metadata
}

// When there is no result from the plugin in global query, Wox will call QueryFallback
type FallbackSearcher interface {
	QueryFallback(ctx context.Context, query Query) []QueryResult
}

// GlanceProvider is implemented by plugins that expose Global Glance items.
// The method is optional so existing plugins keep the minimal Init/Query contract.
type GlanceProvider interface {
	Glance(ctx context.Context, request GlanceRequest) GlanceResponse
}

// ActionProxyCreator is implemented by plugins that need to create proxy callbacks for actions
// This is used by external plugins (Node.js/Python) to create callbacks that invoke the host
type ActionProxyCreator interface {
	CreateActionProxy(actionId string) func(context.Context, ActionContext)
}

// FormActionProxyCreator is implemented by plugins that need to create proxy callbacks for form actions
// This is used by external plugins (Node.js/Python) to create callbacks that invoke the host
type FormActionProxyCreator interface {
	CreateFormActionProxy(actionId string) func(context.Context, FormActionContext)
}

// ToolbarMsgActionProxyCreator is implemented by external plugins that need
// toolbar msg action callbacks to round-trip through the host runtime.
type ToolbarMsgActionProxyCreator interface {
	CreateToolbarMsgActionProxy(actionId string) func(context.Context, ToolbarMsgActionContext)
}

type InitParams struct {
	API             API
	PluginDirectory string
}
