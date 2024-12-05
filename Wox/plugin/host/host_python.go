package host

import (
	"context"
	"fmt"
	"path"
	"strings"
	"wox/plugin"
	"wox/util"

	"github.com/Masterminds/semver/v3"
	"github.com/mitchellh/go-homedir"
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
	return n.websocketHost.StartHost(ctx, n.findPythonPath(ctx), path.Join(util.GetLocation().GetHostDirectory(), "python-host.pyz"))
}

func (n *PythonHost) findPythonPath(ctx context.Context) string {
	util.GetLogger().Debug(ctx, "start finding python path")

	var possiblePythonPaths = []string{
		"/opt/homebrew/bin/python3",
		"/usr/local/bin/python3",
		"/usr/bin/python3",
		"/usr/local/python3",
	}

	pyenvPaths, _ := homedir.Expand("~/.pyenv/versions")
	if util.IsDirExists(pyenvPaths) {
		versions, _ := util.ListDir(pyenvPaths)
		for _, v := range versions {
			possiblePythonPaths = append(possiblePythonPaths, path.Join(pyenvPaths, v, "bin", "python3"))
		}
	}

	foundVersion, _ := semver.NewVersion("v0.0.1")
	foundPath := ""
	for _, p := range possiblePythonPaths {
		if util.IsFileExists(p) {
			versionOriginal, versionErr := util.ShellRunOutput(p, "--version")
			if versionErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("failed to get python version: %s, path=%s", versionErr, p))
				continue
			}
			// Python version output format is like "Python 3.9.0"
			version := strings.TrimSpace(string(versionOriginal))
			version = strings.TrimPrefix(version, "Python ")
			version = "v" + version
			installedVersion, err := semver.NewVersion(version)
			if err != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("failed to parse python version: %s, path=%s", err, p))
				continue
			}
			util.GetLogger().Debug(ctx, fmt.Sprintf("found python path: %s, version: %s", p, installedVersion.String()))

			if installedVersion.GreaterThan(foundVersion) {
				foundPath = p
				foundVersion = installedVersion
			}
		}
	}

	if foundPath != "" {
		util.GetLogger().Info(ctx, fmt.Sprintf("finally use python path: %s, version: %s", foundPath, foundVersion.String()))
		return foundPath
	}

	util.GetLogger().Info(ctx, "finally use default python3 from env path")
	return "python3"
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
