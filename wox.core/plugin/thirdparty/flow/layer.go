// Package flow is the compatibility layer discovered from plugin/thirdparty/flow.
// The shared parser lives in the manifest child package so the script
// and dotnet hosts can use it without an import cycle.
package flow

import (
	"context"

	"wox/plugin"
	"wox/plugin/thirdparty/flow/dotnet"
	"wox/plugin/thirdparty/flow/manifest"
	"wox/plugin/thirdparty/flow/script"
	"wox/plugin/thirdparty/migrate"
)

func init() {
	plugin.RegisterThirdParty(flowLayer{})
	migrate.Register(flowMigrationSource{})
}

// flowLayer is the compatibility entry for this directory.
type flowLayer struct{}

func (flowLayer) Name() string { return "flow" }

func (flowLayer) Directory() string { return manifest.DirectoryName }

func (flowLayer) Hosts() []plugin.Host {
	return []plugin.Host{&script.Host{}, &dotnet.Host{}}
}

// SupportedOS keeps Flow off macOS and Linux. Flow Launcher plugins, including
// the Python and Node plugins in this layer, run only on Windows.
func (flowLayer) SupportedOS() []string { return []string{"Windows"} }

// Uninstall removes one plugin imported into the flow collection directory.
func (flowLayer) Uninstall(ctx context.Context, instance *plugin.Instance, skipCleanSetting bool, preserveCache bool, progress plugin.UninstallProgressCallback) (bool, error) {
	return manifest.Uninstall(ctx, instance, skipCleanSetting, preserveCache, progress)
}

var (
	_ plugin.Layer         = flowLayer{}
	_ plugin.PluginRemover = flowLayer{}
)
