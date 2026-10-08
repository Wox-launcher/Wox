// Package flow is the compatibility layer discovered from plugin/thirdparty/flow.
// The shared parser and store live in the manifest child package so the script
// and dotnet hosts can use them without an import cycle.
package flow

import (
	"wox/plugin"
	"wox/plugin/thirdparty/flow/dotnet"
	"wox/plugin/thirdparty/flow/manifest"
	"wox/plugin/thirdparty/flow/script"
)

func init() {
	plugin.RegisterThirdParty(flowLayer{})
}

// flowLayer is the compatibility entry for this directory.
type flowLayer struct{}

func (flowLayer) Name() string { return "flow" }

func (flowLayer) Directory() string { return manifest.DirectoryName }

func (flowLayer) Hosts() []plugin.Host {
	return []plugin.Host{&script.Host{}, &dotnet.Host{}}
}

func (flowLayer) Store() plugin.ExternalStore { return manifest.Store() }

var _ plugin.Layer = flowLayer{}
