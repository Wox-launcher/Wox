package host

import (
	"context"
	"path"
	"wox/plugin"
	"wox/util"
)

func init() {
	host := &NodejsHost{}
	host.websocketHost = &WebsocketHost{
		host:       host,
		requestMap: util.NewHashMap[string, chan JsonRpcResponse](),
	}
	plugin.AllHosts = append(plugin.AllHosts, host)
}

type NodejsHost struct {
	websocketHost *WebsocketHost
}

func (n *NodejsHost) GetRuntime(ctx context.Context) plugin.Runtime {
	return plugin.PLUGIN_RUNTIME_NODEJS
}

func (n *NodejsHost) Start(ctx context.Context) error {
	return n.websocketHost.StartHost(ctx, n.findNodejsPath(ctx), path.Join(util.GetLocation().GetHostDirectory(), "node-host.js"))
}

func (n *NodejsHost) findNodejsPath(ctx context.Context) string {
	if output, err := util.ShellRunOutput("which", "node"); err == nil {
		return string(output)
	}

	return "node"
}

func (n *NodejsHost) IsStarted(ctx context.Context) bool {
	return n.websocketHost.IsHostStarted(ctx)
}

func (n *NodejsHost) Stop(ctx context.Context) {
	n.websocketHost.StopHost(ctx)
}

func (n *NodejsHost) LoadPlugin(ctx context.Context, metadata plugin.Metadata, pluginDirectory string) (plugin.Plugin, error) {
	return n.websocketHost.LoadPlugin(ctx, metadata, pluginDirectory)
}

func (n *NodejsHost) UnloadPlugin(ctx context.Context, metadata plugin.Metadata) {
	n.websocketHost.UnloadPlugin(ctx, metadata)
}
