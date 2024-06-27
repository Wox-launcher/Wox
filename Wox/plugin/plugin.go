package plugin

import "context"

var AllSystemPlugin []SystemPlugin

type Plugin interface {
	Init(ctx context.Context, initParams InitParams)
	Query(ctx context.Context, query Query) []QueryResult
}

type SystemPlugin interface {
	Plugin
	GetMetadata() Metadata
}

// When there is no result from the plugin in global query, Wox will call QueryFallback
type FallbackSearcher interface {
	QueryFallback(ctx context.Context, query Query) []QueryResult
}

type InitParams struct {
	API             API
	PluginDirectory string
}
