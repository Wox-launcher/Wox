package plugin

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"wox/util"
	"wox/util/trash"
)

func (s *Store) installSingleFilePluginWithProgress(ctx context.Context, manifest StorePluginManifest, progress InstallProgressCallback) error {
	return s.installStagedArtifact(ctx, GetPluginManager().GetPluginInstanceById(manifest.Id), manifest, PluginArtifactSingleFile, "", progress)
}
func (s *Store) uninstallSingleFilePlugin(ctx context.Context, plugin *Instance) error {
	filePath, err := singleFilePluginPath(plugin.Metadata)
	if err != nil {
		return err
	}
	GetPluginManager().UnloadPlugin(ctx, plugin)
	if util.IsFileExists(filePath) {
		if removeErr := trash.MoveToTrash(filePath); removeErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to remove single-file plugin %s: %s", filePath, removeErr.Error()))
			return removeErr
		}
	}
	return nil
}

func findInstalledSingleFilePluginPath(ctx context.Context, pluginID string) (string, error) {
	if instance := GetPluginManager().GetPluginInstanceById(pluginID); instance != nil && IsSingleFilePlugin(instance.Metadata) {
		return singleFilePluginPath(instance.Metadata)
	}

	directory := util.GetLocation().GetUserSingleFilePluginsDirectory()
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read single-file plugin directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || shouldIgnoreSingleFileWatchName(entry.Name()) || !isSingleFilePluginSourceName(entry.Name()) {
			continue
		}
		filePath := path.Join(directory, entry.Name())
		metadata, metaErr := GetPluginManager().ParseSingleFilePluginMetadata(ctx, filePath)
		if metaErr != nil {
			continue
		}
		if strings.EqualFold(metadata.Id, pluginID) {
			return filePath, nil
		}
	}
	return "", nil
}

func findSingleFilePluginIDAtPath(ctx context.Context, filePath string) (string, bool) {
	if !util.IsFileExists(filePath) {
		return "", false
	}
	metadata, err := GetPluginManager().ParseSingleFilePluginMetadata(ctx, filePath)
	if err != nil {
		return "", true
	}
	return metadata.Id, true
}

func copyFile(src string, dest string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destination.Close()

	if _, err := io.Copy(destination, source); err != nil {
		return err
	}
	if info, statErr := os.Stat(src); statErr == nil {
		_ = os.Chmod(dest, info.Mode())
	}
	return nil
}
