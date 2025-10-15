package host

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"wox/plugin"
	"wox/setting"
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

	// Check if user has configured a custom Node.js path
	customPath := setting.GetSettingManager().GetWoxSetting(ctx).CustomNodejsPath.Get()
	if customPath != "" {
		if util.IsFileExists(customPath) {
			util.GetLogger().Info(ctx, fmt.Sprintf("using custom nodejs path: %s", customPath))
			return customPath
		} else {
			util.GetLogger().Warn(ctx, fmt.Sprintf("custom nodejs path not found, falling back to auto-detection: %s", customPath))
		}
	}

	possibleNodejsPaths := collectNodejsPaths()

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

func collectNodejsPaths() []string {
	switch runtime.GOOS {
	case "windows":
		return collectNodejsPathsForWindows()
	case "darwin":
		return collectNodejsPathsForDarwin()
	default:
		return collectNodejsPathsForLinux()
	}
}

func collectNodejsPathsForDarwin() []string {
	paths := []string{
		"/opt/homebrew/bin/node",
		"/usr/local/bin/node",
		"/usr/bin/node",
		"/usr/local/node",
	}
	paths = append(paths, collectNodejsPathsFromNvmUnix()...)
	paths = append(paths, collectVoltaNodePaths()...)
	return util.UniqueStrings(paths)
}

func collectNodejsPathsForLinux() []string {
	paths := []string{
		"/usr/local/bin/node",
		"/usr/bin/node",
		"/usr/local/node",
	}
	paths = append(paths, collectNodejsPathsFromNvmUnix()...)
	paths = append(paths, collectVoltaNodePaths()...)
	return util.UniqueStrings(paths)
}

func collectNodejsPathsForWindows() []string {
	var candidates []string
	binaries := []string{"node.exe"}

	if nvmHome := os.Getenv("NVM_HOME"); nvmHome != "" {
		candidates = append(candidates, util.CollectExecutables(nvmHome, binaries, func(name string) bool {
			return strings.HasPrefix(strings.ToLower(name), "v")
		})...)
	}

	if nvmSymlink := os.Getenv("NVM_SYMLINK"); nvmSymlink != "" {
		for _, binary := range binaries {
			candidates = append(candidates, filepath.Join(nvmSymlink, binary))
		}
	}

	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		candidates = append(candidates, util.CollectExecutables(filepath.Join(localAppData, "Programs", "nodejs"), binaries, nil)...)
	}

	for _, envVar := range []string{"PROGRAMFILES", "PROGRAMFILES(X86)"} {
		if base := os.Getenv(envVar); base != "" {
			candidates = append(candidates, filepath.Join(base, "nodejs", "node.exe"))
		}
	}

	if homeDir, err := homedir.Dir(); err == nil {
		for _, scoopPackage := range []string{"nodejs", "nodejs-lts"} {
			candidates = append(candidates, util.CollectExecutables(filepath.Join(homeDir, "scoop", "apps", scoopPackage), binaries, nil)...)
		}
	}

	candidates = append(candidates, collectVoltaNodePaths()...)
	return util.UniqueStrings(candidates)
}

func collectNodejsPathsFromNvmUnix() []string {
	nvmDir := os.Getenv("NVM_DIR")
	if nvmDir == "" {
		var err error
		nvmDir, err = homedir.Expand("~/.nvm")
		if err != nil {
			return nil
		}
	}

	nodeVersions := filepath.Join(nvmDir, "versions", "node")
	if !util.IsDirExists(nodeVersions) {
		return nil
	}

	versions, err := util.ListDir(nodeVersions)
	if err != nil {
		return nil
	}

	var paths []string
	for _, v := range versions {
		paths = append(paths, filepath.Join(nodeVersions, v, "bin", "node"))
	}

	return paths
}

func collectVoltaNodePaths() []string {
	voltaHome := os.Getenv("VOLTA_HOME")
	if voltaHome == "" {
		if homeDir, err := homedir.Dir(); err == nil {
			voltaHome = filepath.Join(homeDir, ".volta")
		}
	}

	if voltaHome == "" {
		return nil
	}

	binaryName := "node"
	if runtime.GOOS == "windows" {
		binaryName = "node.exe"
	}

	return []string{filepath.Join(voltaHome, "bin", binaryName)}
}
