package host

import (
	"context"
	"fmt"
	"path"
	"strings"
	"wox/plugin"
	"wox/util"
	"wox/util/shell"

	"github.com/Masterminds/semver/v3"
	"github.com/mitchellh/go-homedir"
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
	return n.websocketHost.StartHost(ctx, n.findNodejsPath(ctx), path.Join(util.GetLocation().GetHostDirectory(), "node-host.js"), nil)
}

func (n *NodejsHost) findNodejsPath(ctx context.Context) string {
	util.GetLogger().Debug(ctx, "start finding nodejs path")

	var possibleNodejsPaths = []string{
		"/opt/homebrew/bin/node",
		"/usr/local/bin/node",
		"/usr/bin/node",
		"/usr/local/node",
	}

	nvmNodePaths, _ := homedir.Expand("~/.nvm/versions/node")
	if util.IsDirExists(nvmNodePaths) {
		versions, _ := util.ListDir(nvmNodePaths)
		for _, v := range versions {
			possibleNodejsPaths = append(possibleNodejsPaths, path.Join(nvmNodePaths, v, "bin", "node"))
		}
	}

	foundVersion, _ := semver.NewVersion("v0.0.1")
	foundPath := ""
	for _, p := range possibleNodejsPaths {
		if util.IsFileExists(p) {
			versionOriginal, versionErr := shell.RunOutput(p, "-v")
			if versionErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("failed to get nodejs version: %s, path=%s", versionErr, p))
				continue
			}
			version := strings.TrimSpace(string(versionOriginal))
			installedVersion, _ := semver.NewVersion(version)
			util.GetLogger().Debug(ctx, fmt.Sprintf("found nodejs path: %s, version: %s", p, installedVersion.String()))

			if installedVersion.GreaterThan(foundVersion) {
				foundPath = p
				foundVersion = installedVersion
			}
		}
	}

	if foundPath != "" {
		util.GetLogger().Info(ctx, fmt.Sprintf("finally use nodejs path: %s, version: %s", foundPath, foundVersion.String()))
		return foundPath
	}

	util.GetLogger().Info(ctx, "finally use default node from env path")
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
