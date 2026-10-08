// Package dotnet is the flow host for C# and F# plugins.
// It is not registered on plugin.AllHosts. Those plugins need their own process
// host, which is separate from the script JSON-RPC host.
package dotnet

import (
	"context"
	"errors"

	"wox/plugin"
	"wox/plugin/thirdparty/flow/manifest"
)

// Host will run C# and F# plugins. Callers must not add it to plugin.AllHosts
// until LoadPlugin can start them.
type Host struct{}

func (h *Host) GetRuntime(ctx context.Context) plugin.Runtime {
	return manifest.RuntimeDotNet
}

func (h *Host) Start(ctx context.Context) error { return nil }

func (h *Host) Stop(ctx context.Context) {}

func (h *Host) IsStarted(ctx context.Context) bool { return false }

func (h *Host) RuntimeStatus(ctx context.Context) plugin.RuntimeHostStatus {
	return plugin.RuntimeHostStatus{
		StatusCode:    plugin.RuntimeHostStatusStopped,
		StatusMessage: ".NET flow plugins are not supported yet.",
	}
}

// DiscoverMetadata stays empty so registering this host cannot advertise
// plugins it is not able to start. MetadataFrom is the scanner to use then.
func (h *Host) DiscoverMetadata(ctx context.Context) ([]plugin.Metadata, error) {
	return nil, nil
}

func (h *Host) LoadPlugin(ctx context.Context, metadata plugin.Metadata, pluginDirectory string) (plugin.Plugin, error) {
	return nil, errors.New(".NET flow plugins are not supported yet")
}

func (h *Host) UnloadPlugin(ctx context.Context, metadata plugin.Metadata) {}

// MetadataFrom lists C# and F# plugins under root.
func MetadataFrom(ctx context.Context, root string) ([]plugin.Metadata, error) {
	descriptors, err := manifest.LoadDirectory(ctx, root)
	if err != nil {
		return nil, err
	}
	var metadata []plugin.Metadata
	for _, descriptor := range descriptors {
		if descriptor.Kind == manifest.KindDotNet {
			metadata = append(metadata, descriptor.Metadata)
		}
	}
	return metadata, nil
}
