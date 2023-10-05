package plugin

import (
	"context"
)

var AllHosts []Host

type Host interface {
	GetRuntime(ctx context.Context) Runtime
	Start(ctx context.Context) error
	Stop(ctx context.Context)
	LoadPlugin(ctx context.Context, metadata Metadata, pluginDirectory string) (Plugin, error)
	UnloadPlugin(ctx context.Context, metadata Metadata)
}
