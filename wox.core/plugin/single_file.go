package plugin

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"wox/util"
)

const (
	userScriptPluginsDirName     = "scripts"
	userSingleFilePluginsDirName = "single-file"
	singleFileWatchDebounce      = 500 * time.Millisecond
	singleFileReadRetryWindow    = time.Second
	singleFileManagedWriteWindow = 2 * time.Second
)

// IsReservedUserPluginDirectory reports directories that must not be scanned as packaged plugins.
func IsReservedUserPluginDirectory(name string) bool {
	return name == userScriptPluginsDirName || name == userSingleFilePluginsDirName
}

// IsSingleFilePlugin reports whether metadata points at the shared single-file directory.
func IsSingleFilePlugin(metadata Metadata) bool {
	return IsSingleFilePluginDirectory(metadata.Directory)
}

// IsSingleFilePluginDirectory compares a cleaned plugin directory with the shared single-file root.
func IsSingleFilePluginDirectory(directory string) bool {
	return pluginPathsEqual(directory, util.GetLocation().GetUserSingleFilePluginsDirectory())
}

func pluginPathsEqual(left string, right string) bool {
	leftClean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(left)))
	rightClean := filepath.Clean(filepath.FromSlash(strings.TrimSpace(right)))
	if leftClean == "." || rightClean == "." || leftClean == "" || rightClean == "" {
		return false
	}
	if leftAbs, err := filepath.Abs(leftClean); err == nil {
		leftClean = leftAbs
	}
	if rightAbs, err := filepath.Abs(rightClean); err == nil {
		rightClean = rightAbs
	}
	return strings.EqualFold(leftClean, rightClean)
}

func singleFileRuntimeForPath(filePath string) (Runtime, error) {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".py":
		return PLUGIN_RUNTIME_PYTHON, nil
	case ".js":
		return PLUGIN_RUNTIME_NODEJS, nil
	default:
		return "", fmt.Errorf("unsupported single-file plugin extension: %s", filepath.Ext(filePath))
	}
}

func isSingleFilePluginSourceName(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".py" || ext == ".js"
}

func shouldIgnoreSingleFileWatchName(name string) bool {
	if name == "" || name == ".DS_Store" || name == "README.md" {
		return true
	}
	if strings.HasPrefix(name, ".") {
		return true
	}
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, "~") || strings.HasSuffix(lower, ".tmp") || strings.HasSuffix(lower, ".bak") || strings.HasSuffix(lower, ".swp") {
		return true
	}
	return false
}

func isPathInsideDirectory(filePath string, directory string) error {
	fileAbs, err := filepath.Abs(filePath)
	if err != nil {
		return err
	}
	dirAbs, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(dirAbs, fileAbs)
	if err != nil {
		return fmt.Errorf("plugin file %s is outside %s", fileAbs, dirAbs)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("plugin file %s is outside %s", fileAbs, dirAbs)
	}
	return nil
}

func singleFilePluginPath(metadata Metadata) (string, error) {
	if metadata.Entry == "" {
		return "", fmt.Errorf("single-file plugin %s is missing Entry", metadata.Id)
	}
	directory := metadata.Directory
	if directory == "" {
		directory = util.GetLocation().GetUserSingleFilePluginsDirectory()
	}
	filePath := filepath.Join(directory, metadata.Entry)
	if err := isPathInsideDirectory(filePath, util.GetLocation().GetUserSingleFilePluginsDirectory()); err != nil {
		return "", err
	}
	return filePath, nil
}

// ensureSingleFilePluginIDAvailable prevents a hot reload from replacing a different plugin with the same ID.
func (m *Manager) ensureSingleFilePluginIDAvailable(metadata Metadata) error {
	filePath, err := singleFilePluginPath(metadata)
	if err != nil {
		return err
	}
	for _, instance := range m.pluginInstancesSnapshot() {
		if !strings.EqualFold(instance.Metadata.Id, metadata.Id) {
			continue
		}
		existingPath, pathErr := singleFilePluginPath(instance.Metadata)
		if pathErr == nil && pluginPathsEqual(existingPath, filePath) {
			continue
		}
		return fmt.Errorf("plugin ID %s is already loaded from %s", metadata.Id, instance.Metadata.Directory)
	}
	return nil
}
