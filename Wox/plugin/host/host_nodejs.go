package host

import (
	"context"
	"wox/plugin"
)

func init() {
	host := &NodejsHost{}
	host.WebsocketHost.this = host
	plugin.AllHosts = append(plugin.AllHosts, host)
}

type NodejsHost struct {
	WebsocketHost
}

func (n *NodejsHost) GetRuntime(ctx context.Context) plugin.Runtime {
	return plugin.PLUGIN_RUNTIME_NODEJS
}

func (n *NodejsHost) Start(ctx context.Context) error {
	return n.StartHost(ctx, "/opt/homebrew/bin/node", "/Users/s/.wox/Plugins/nodejs/wox.js")
}

func (n *NodejsHost) Stop(ctx context.Context) {

}

func (n *NodejsHost) LoadPlugin(ctx context.Context, metadata plugin.Metadata, pluginDirectory string) (plugin.Plugin, error) {
	return nil, nil
}

func (n *NodejsHost) UnloadPlugin(ctx context.Context, metadata plugin.Metadata) {

}
