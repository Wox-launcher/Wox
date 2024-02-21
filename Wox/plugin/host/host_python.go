package host

import (
	"context"
	"path"
	"wox/plugin"
	"wox/util"
)

func init() {
	host := &PythonHost{}
	host.websocketHost = &WebsocketHost{
		host:       host,
		requestMap: util.NewHashMap[string, chan JsonRpcResponse](),
	}
	plugin.AllHosts = append(plugin.AllHosts, host)
}

type PythonHost struct {
	websocketHost *WebsocketHost
}

func (n *PythonHost) GetRuntime(ctx context.Context) plugin.Runtime {
	return plugin.PLUGIN_RUNTIME_PYTHON
}

func (n *PythonHost) Start(ctx context.Context) error {
	return n.websocketHost.StartHost(ctx, "python", path.Join(util.GetLocation().GetHostDirectory(), "python-host.pyz"))
}

func (n *PythonHost) IsStarted(ctx context.Context) bool {
	return n.websocketHost.IsHostStarted(ctx)
}

func (n *PythonHost) Stop(ctx context.Context) {
	n.websocketHost.StopHost(ctx)
}

func (n *PythonHost) LoadPlugin(ctx context.Context, metadata plugin.Metadata, pluginDirectory string) (plugin.Plugin, error) {
	return n.websocketHost.LoadPlugin(ctx, metadata, pluginDirectory)
}

func (n *PythonHost) UnloadPlugin(ctx context.Context, metadata plugin.Metadata) {
	n.websocketHost.UnloadPlugin(ctx, metadata)
}
