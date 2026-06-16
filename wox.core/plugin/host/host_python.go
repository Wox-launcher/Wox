package host

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
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

const pythonInstallUrl = "https://www.python.org/downloads/"

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
	pythonPath, pythonErr := n.resolvePythonPath(ctx)
	if pythonErr != nil {
		return pythonErr
	}

	return n.websocketHost.StartHost(ctx, pythonPath, path.Join(util.GetLocation().GetHostDirectory(), "python-host.pyz"), []string{"SHIV_ROOT=" + util.GetLocation().GetCacheDirectory()})
}

// FindPythonPath finds the best available Python interpreter path
// It checks custom path first, then auto-detects from common installation locations
func FindPythonPath(ctx context.Context) string {
	pythonPath, err := (&PythonHost{}).resolvePythonPath(ctx)
	if err != nil {
		return defaultPythonExecutableNames()[0]
	}
	return pythonPath
}

// minimumPythonVersion is the minimum Python version required by Wox.
// Bug fix: the Python host package declares Python 3.10+ and bundled
// dependencies use syntax that Python 3.9 cannot parse. The old 3.9 floor let
// an incompatible executable pass discovery and then fail during host startup.
var minimumPythonVersion, _ = semver.NewVersion("v3.10.0")

// ValidatePythonExecutable verifies that a custom Python executable can run the Wox host.
func ValidatePythonExecutable(ctx context.Context, pythonPath string) (*semver.Version, error) {
	normalizedPath := strings.TrimSpace(pythonPath)
	if normalizedPath == "" {
		message := "Python executable path is empty."
		return nil, &runtimeExecutableError{statusCode: plugin.RuntimeHostStatusExecutableMissing, message: message}
	}

	// Bug fix: custom Python paths are now validated before they are persisted.
	// The previous setting flow only checked the path during host startup, which
	// let users save Python 3.9 and then see a later startup failure from the
	// Python package. Keeping this helper in the host package makes the settings
	// API and host resolver enforce the same executable and minimum-version rules.
	if !util.IsFileExists(normalizedPath) {
		message := fmt.Sprintf("custom Python path does not exist: %s", normalizedPath)
		util.GetLogger().Warn(ctx, message)
		return nil, &runtimeExecutableError{statusCode: plugin.RuntimeHostStatusExecutableMissing, message: message, path: normalizedPath}
	}

	installedVersion, versionErr := getPythonExecutableVersion(ctx, normalizedPath)
	if versionErr != nil {
		return nil, fmt.Errorf("failed to get custom Python version at %s: %w", normalizedPath, versionErr)
	}
	if installedVersion.LessThan(minimumPythonVersion) {
		message := fmt.Sprintf("Python %s at %s is below the minimum required version %s.", installedVersion.String(), normalizedPath, minimumPythonVersion.String())
		util.GetLogger().Warn(ctx, message)
		return nil, &runtimeExecutableError{statusCode: plugin.RuntimeHostStatusUnsupportedVersion, message: message, path: normalizedPath}
	}

	return installedVersion, nil
}

func (n *PythonHost) resolvePythonPath(ctx context.Context) (string, error) {
	util.GetLogger().Debug(ctx, "start finding python path")

	// Bug fix: a broken custom path is a user-actionable configuration problem,
	// not a generic host startup failure, so do not hide it behind auto-detect.
	customPath := setting.GetSettingManager().GetWoxSetting(ctx).CustomPythonPath.Get()
	if customPath != "" {
		installedVersion, validateErr := ValidatePythonExecutable(ctx, customPath)
		if validateErr != nil {
			return "", validateErr
		}

		util.GetLogger().Info(ctx, fmt.Sprintf("using custom python path: %s, version: %s", customPath, installedVersion.String()))
		return customPath, nil
	}

	possiblePythonPaths := collectPythonPaths()

	foundVersion, _ := semver.NewVersion("v0.0.1")
	foundPath := ""
	var unsupportedPath string
	var unsupportedVersion *semver.Version
	for _, p := range possiblePythonPaths {
		if util.IsFileExists(p) {
			installedVersion, versionErr := getPythonExecutableVersion(ctx, p)
			if versionErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("failed to get python version: %s, path=%s", versionErr, p))
				continue
			}
			util.GetLogger().Debug(ctx, fmt.Sprintf("found python path: %s, version: %s", p, installedVersion.String()))

			// Feature: preserve the best unsupported version so the UI can guide
			// users to upgrade Python instead of reporting a vague missing host.
			if installedVersion.LessThan(minimumPythonVersion) {
				util.GetLogger().Warn(ctx, fmt.Sprintf("skipping python %s at %s: version is below minimum required %s, please upgrade your Python installation", installedVersion.String(), p, minimumPythonVersion.String()))
				if unsupportedVersion == nil || installedVersion.GreaterThan(unsupportedVersion) {
					unsupportedPath = p
					unsupportedVersion = installedVersion
				}
				continue
			}

			if installedVersion.GreaterThan(foundVersion) {
				foundPath = p
				foundVersion = installedVersion
			}
		}
	}

	if foundPath != "" {
		util.GetLogger().Info(ctx, fmt.Sprintf("finally use python path: %s, version: %s", foundPath, foundVersion.String()))
		return foundPath, nil
	}

	for _, executableName := range defaultPythonExecutableNames() {
		envPath, lookErr := exec.LookPath(executableName)
		if lookErr != nil {
			continue
		}
		installedVersion, versionErr := getPythonExecutableVersion(ctx, envPath)
		if versionErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to get python version from env path: %s, path=%s", versionErr, envPath))
			continue
		}
		if installedVersion.LessThan(minimumPythonVersion) {
			if unsupportedVersion == nil || installedVersion.GreaterThan(unsupportedVersion) {
				unsupportedPath = envPath
				unsupportedVersion = installedVersion
			}
			continue
		}

		util.GetLogger().Info(ctx, fmt.Sprintf("finally use python path from env: %s, version: %s", envPath, installedVersion.String()))
		return envPath, nil
	}

	if unsupportedVersion != nil {
		message := fmt.Sprintf("Python %s at %s is below the minimum required version %s.", unsupportedVersion.String(), unsupportedPath, minimumPythonVersion.String())
		util.GetLogger().Warn(ctx, message)
		return "", &runtimeExecutableError{statusCode: plugin.RuntimeHostStatusUnsupportedVersion, message: message, path: unsupportedPath}
	}

	message := "Python executable was not found. Install Python or configure the Python path in runtime settings."
	util.GetLogger().Warn(ctx, message)
	return "", &runtimeExecutableError{statusCode: plugin.RuntimeHostStatusExecutableMissing, message: message}
}

func (n *PythonHost) IsStarted(ctx context.Context) bool {
	return n.websocketHost.IsHostStarted(ctx)
}

func (n *PythonHost) RuntimeStatus(ctx context.Context) plugin.RuntimeHostStatus {
	if n.IsStarted(ctx) {
		return plugin.RuntimeHostStatus{
			StatusCode:     plugin.RuntimeHostStatusRunning,
			StatusMessage:  "Python host is running.",
			ExecutablePath: n.websocketHost.GetExecutablePath(),
			CanRestart:     true,
			InstallUrl:     pythonInstallUrl,
		}
	}

	pythonPath, resolveErr := n.resolvePythonPath(ctx)
	if resolveErr != nil {
		var executableErr *runtimeExecutableError
		if errors.As(resolveErr, &executableErr) {
			return plugin.RuntimeHostStatus{
				StatusCode:     executableErr.statusCode,
				StatusMessage:  executableErr.message,
				ExecutablePath: executableErr.path,
				LastStartError: executableErr.message,
				CanRestart:     false,
				InstallUrl:     pythonInstallUrl,
			}
		}
		return plugin.RuntimeHostStatus{
			StatusCode:     plugin.RuntimeHostStatusStartFailed,
			StatusMessage:  "Python host status could not be resolved.",
			LastStartError: resolveErr.Error(),
			CanRestart:     false,
			InstallUrl:     pythonInstallUrl,
		}
	}

	if lastStartError := n.websocketHost.GetLastStartError(); lastStartError != "" {
		return plugin.RuntimeHostStatus{
			StatusCode:     plugin.RuntimeHostStatusStartFailed,
			StatusMessage:  "Python host failed to start.",
			ExecutablePath: pythonPath,
			LastStartError: lastStartError,
			CanRestart:     true,
			InstallUrl:     pythonInstallUrl,
		}
	}

	return plugin.RuntimeHostStatus{
		StatusCode:     plugin.RuntimeHostStatusStopped,
		StatusMessage:  "Python host is not running.",
		ExecutablePath: pythonPath,
		CanRestart:     true,
		InstallUrl:     pythonInstallUrl,
	}
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

func getPythonExecutableVersion(ctx context.Context, pythonPath string) (*semver.Version, error) {
	versionOriginal, versionErr := shell.RunOutput(pythonPath, "--version")
	if versionErr != nil {
		return nil, versionErr
	}

	// Python version output format is like "Python 3.9.0". Normalizing here
	// keeps all resolver branches on the same minimum-version comparison.
	version := strings.TrimSpace(string(versionOriginal))
	version = strings.TrimPrefix(version, "Python ")
	version = "v" + version
	installedVersion, err := semver.NewVersion(version)
	if err != nil {
		return nil, err
	}

	util.GetLogger().Debug(ctx, fmt.Sprintf("resolved python version: %s, path=%s", installedVersion.String(), pythonPath))
	return installedVersion, nil
}

func defaultPythonExecutableNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"python", "python3"}
	}

	return []string{"python3", "python"}
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
