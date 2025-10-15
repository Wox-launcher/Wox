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
	return n.websocketHost.StartHost(ctx, n.findPythonPath(ctx), path.Join(util.GetLocation().GetHostDirectory(), "python-host.pyz"), []string{"SHIV_ROOT=" + util.GetLocation().GetCacheDirectory()})
}

func (n *PythonHost) findPythonPath(ctx context.Context) string {
	util.GetLogger().Debug(ctx, "start finding python path")

	// Check if user has configured a custom Python path
	customPath := setting.GetSettingManager().GetWoxSetting(ctx).CustomPythonPath.Get()
	if customPath != "" {
		if util.IsFileExists(customPath) {
			util.GetLogger().Info(ctx, fmt.Sprintf("using custom python path: %s", customPath))
			return customPath
		} else {
			util.GetLogger().Warn(ctx, fmt.Sprintf("custom python path not found, falling back to auto-detection: %s", customPath))
		}
	}

	possiblePythonPaths := collectPythonPaths()

	foundVersion, _ := semver.NewVersion("v0.0.1")
	foundPath := ""
	for _, p := range possiblePythonPaths {
		if util.IsFileExists(p) {
			versionOriginal, versionErr := shell.RunOutput(p, "--version")
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

	defaultPython := "python3"
	if runtime.GOOS == "windows" {
		defaultPython = "python"
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("finally use default %s from env path", defaultPython))
	return defaultPython
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

func collectPythonPaths() []string {
	switch runtime.GOOS {
	case "windows":
		return collectPythonPathsForWindows()
	case "darwin":
		return collectPythonPathsForDarwin()
	default:
		return collectPythonPathsForLinux()
	}
}

func collectPythonPathsForDarwin() []string {
	paths := []string{
		"/opt/homebrew/bin/python3",
		"/usr/local/bin/python3",
		"/usr/bin/python3",
		"/usr/local/python3",
	}
	paths = append(paths, collectPythonPathsFromPyenvUnix()...)
	return util.UniqueStrings(paths)
}

func collectPythonPathsForLinux() []string {
	paths := []string{
		"/usr/local/bin/python3",
		"/usr/bin/python3",
		"/usr/local/python3",
	}
	paths = append(paths, collectPythonPathsFromPyenvUnix()...)
	return util.UniqueStrings(paths)
}

func collectPythonPathsForWindows() []string {
	var candidates []string
	binaries := []string{"python.exe", "python3.exe"}

	if pythonHome := os.Getenv("PYTHONHOME"); pythonHome != "" {
		for _, binary := range binaries {
			candidates = append(candidates, filepath.Join(pythonHome, binary))
		}
	}

	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		candidates = append(candidates, util.CollectExecutables(filepath.Join(localAppData, "Programs", "Python"), binaries, nil)...)
	}

	for _, envVar := range []string{"PROGRAMFILES", "PROGRAMFILES(X86)"} {
		if base := os.Getenv(envVar); base != "" {
			candidates = append(candidates, util.CollectExecutables(base, binaries, func(name string) bool {
				return strings.HasPrefix(strings.ToLower(name), "python")
			})...)
		}
	}

	if homeDir, err := homedir.Dir(); err == nil {
		candidates = append(candidates, util.CollectExecutables(filepath.Join(homeDir, "scoop", "apps", "python"), binaries, nil)...)
	}

	candidates = append(candidates, collectPythonPathsFromPyenvWin()...)
	return util.UniqueStrings(candidates)
}

func collectPythonPathsFromPyenvUnix() []string {
	pyenvPaths, _ := homedir.Expand("~/.pyenv/versions")
	if !util.IsDirExists(pyenvPaths) {
		return nil
	}

	versions, err := util.ListDir(pyenvPaths)
	if err != nil {
		return nil
	}

	var paths []string
	for _, v := range versions {
		paths = append(paths, filepath.Join(pyenvPaths, v, "bin", "python3"))
	}

	return paths
}

func collectPythonPathsFromPyenvWin() []string {
	pyenvWinPaths, _ := homedir.Expand("~/.pyenv/pyenv-win/versions")
	if !util.IsDirExists(pyenvWinPaths) {
		return nil
	}

	versions, err := util.ListDir(pyenvWinPaths)
	if err != nil {
		return nil
	}

	var paths []string
	for _, v := range versions {
		paths = append(paths, filepath.Join(pyenvWinPaths, v, "python.exe"))
		paths = append(paths, filepath.Join(pyenvWinPaths, v, "python3.exe"))
	}

	return paths
}
