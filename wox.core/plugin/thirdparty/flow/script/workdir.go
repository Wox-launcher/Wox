package script

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"wox/plugin/thirdparty/flow/manifest"
	"wox/util"
)

const flowLauncherSettings = `{"PluginSettings":{"Plugins":{},"PythonDirectory":""}}`

// processDir is the directory the plugin process starts in.
// Python plugins start through a link whose parents are FlowLauncher/UserData
// and that contains Settings. Clients locate the launcher by walking that
// shape, and the real plugin directory does not have it. Node and executable
// plugins keep their own directory.
func (s *flowSession) processDir() string {
	if s.language != "python" {
		return s.directory
	}
	root := strings.TrimSpace(s.launcherRoot)
	if root == "" {
		collection := filepath.Clean(manifest.CollectionDirectory())
		relative, err := filepath.Rel(collection, filepath.Clean(s.directory))
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return s.directory
		}
		root = filepath.Join(collection, ".flow-runtime")
	}
	linked, err := flowLauncherWorkDir(root, s.directory)
	if err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("[flow-jsonrpc:%s] launcher working directory: %s", s.name, err.Error()))
		return s.directory
	}
	return linked
}

// flowLauncherWorkDir prepares the launcher directory shape and returns a link
// to pluginDirectory. The link's parents are FlowLauncher/UserData.
func flowLauncherWorkDir(runtimeRoot, pluginDirectory string) (string, error) {
	pluginDirectory, err := filepath.Abs(pluginDirectory)
	if err != nil {
		return "", err
	}
	name := filepath.Base(pluginDirectory)
	if name == "." || name == string(filepath.Separator) || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("plugin directory %s has no usable name", pluginDirectory)
	}
	userData := filepath.Join(runtimeRoot, "FlowLauncher", "UserData")
	settingsDir := filepath.Join(userData, "Settings")
	if err := os.MkdirAll(filepath.Join(settingsDir, "Plugins"), 0o755); err != nil {
		return "", err
	}
	settingsFile := filepath.Join(settingsDir, "Settings.json")
	if _, err := os.Stat(settingsFile); err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		if err := os.WriteFile(settingsFile, []byte(flowLauncherSettings), 0o644); err != nil {
			return "", err
		}
	}
	link := filepath.Join(userData, "Plugins", name)
	if err := ensureDirectoryLink(link, pluginDirectory); err != nil {
		return "", err
	}
	return link, nil
}

func sameDirectoryPath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}
