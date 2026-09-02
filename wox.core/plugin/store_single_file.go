package plugin

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"wox/i18n"
	"wox/util"
	"wox/util/trash"
)

func (s *Store) installSingleFilePluginWithProgress(ctx context.Context, manifest StorePluginManifest, progressCallback InstallProgressCallback) error {
	logger.Info(ctx, fmt.Sprintf("detected single-file plugin, use single-file install flow: %s", manifest.GetName(ctx)))
	if progressCallback != nil {
		progressCallback(i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_install_progress_preparing"))
	}

	directory := util.GetLocation().GetUserSingleFilePluginsDirectory()
	if err := util.GetLocation().EnsureDirectoryExist(directory); err != nil {
		return fmt.Errorf("failed to ensure single-file plugin directory %s: %w", directory, err)
	}

	existingPath, err := findInstalledSingleFilePluginPath(ctx, manifest.Id)
	if err != nil {
		return err
	}

	fileName := artifactFileName(manifest.DownloadUrl)
	if fileName == "" {
		if existingPath != "" {
			fileName = filepath.Base(existingPath)
		} else {
			return fmt.Errorf("single-file plugin download URL has no file name: %s", manifest.DownloadUrl)
		}
	}
	targetPath := path.Join(directory, fileName)
	if err := isPathInsideDirectory(targetPath, directory); err != nil {
		return err
	}

	if existingPath == "" {
		if conflictID, conflict := findSingleFilePluginIDAtPath(ctx, targetPath); conflict && !strings.EqualFold(conflictID, manifest.Id) {
			return fmt.Errorf("single-file plugin file %s is already used by plugin %s", fileName, conflictID)
		}
	} else if !pluginPathsEqual(existingPath, targetPath) {
		if util.IsFileExists(targetPath) {
			return fmt.Errorf("cannot update plugin %s: target file %s already exists", manifest.Id, fileName)
		}
	}

	tempFile, err := os.CreateTemp(directory, ".wox-single-file-download-*"+filepath.Ext(fileName))
	if err != nil {
		return fmt.Errorf("failed to create temporary download file: %w", err)
	}
	tempPath := tempFile.Name()
	_ = tempFile.Close()

	cleanupTemp := func() {
		_ = os.Remove(tempPath)
	}

	logger.Info(ctx, fmt.Sprintf("start to download single-file plugin: %s", manifest.DownloadUrl))
	if progressCallback != nil {
		progressCallback(i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_install_progress_starting_download"))
	}
	downloadErr := downloadPluginFile(ctx, manifest.DownloadUrl, tempPath, pluginDownloadTimeout, func(downloaded int64, total int64) {
		if progressCallback != nil {
			if total > 0 {
				percentage := float64(downloaded) / float64(total) * 100
				progressCallback(fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_install_progress_downloading"), percentage))
			} else {
				progressCallback(fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_install_progress_downloaded_bytes"), downloaded))
			}
		}
	})
	if downloadErr != nil {
		cleanupTemp()
		return fmt.Errorf("failed to download single-file plugin %s(%s): %s", manifest.GetName(ctx), manifest.Version, downloadErr.Error())
	}
	if progressCallback != nil {
		progressCallback(i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_install_progress_download_complete"))
		progressCallback(i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_install_progress_parsing"))
	}

	metadata, metaErr := GetPluginManager().ParseSingleFilePluginMetadata(ctx, tempPath)
	if metaErr != nil {
		cleanupTemp()
		return fmt.Errorf("failed to parse single-file plugin metadata: %s", metaErr.Error())
	}
	metadata.Entry = fileName
	metadata.Directory = directory
	if err := ValidateSingleFileHeaderMatchesManifest(metadata, manifest); err != nil {
		cleanupTemp()
		return err
	}
	expectedRuntime, runtimeErr := singleFileRuntimeForPath(fileName)
	if runtimeErr != nil {
		cleanupTemp()
		return runtimeErr
	}
	if ConvertToRuntime(metadata.Runtime) != expectedRuntime {
		cleanupTemp()
		return fmt.Errorf("single-file plugin suffix %s does not match Runtime %s", filepath.Ext(fileName), metadata.Runtime)
	}

	backupPath := ""
	pluginManager := GetPluginManager()
	pluginManager.IgnoreSingleFileWatch(targetPath)
	if existingPath != "" {
		pluginManager.IgnoreSingleFileWatch(existingPath)
		backupPath = existingPath + ".bak"
		if err := os.Rename(existingPath, backupPath); err != nil {
			if copyErr := copyFile(existingPath, backupPath); copyErr != nil {
				cleanupTemp()
				return fmt.Errorf("failed to backup existing single-file plugin: %w", copyErr)
			}
			if removeErr := os.Remove(existingPath); removeErr != nil {
				cleanupTemp()
				_ = os.Remove(backupPath)
				return fmt.Errorf("failed to move existing single-file plugin aside: %w", removeErr)
			}
		}
		if instance := pluginManager.GetPluginInstanceById(manifest.Id); instance != nil {
			pluginManager.UnloadPlugin(ctx, instance)
		}
	}

	restoreBackup := func() {
		pluginManager.IgnoreSingleFileWatch(targetPath)
		_ = os.Remove(targetPath)
		if backupPath != "" && util.IsFileExists(backupPath) {
			pluginManager.IgnoreSingleFileWatch(existingPath)
			_ = os.Rename(backupPath, existingPath)
			if metadata, parseErr := pluginManager.ParseSingleFilePluginMetadata(ctx, existingPath); parseErr == nil {
				_ = pluginManager.ReloadPlugin(ctx, metadata)
			}
		}
	}

	if err := os.Rename(tempPath, targetPath); err != nil {
		if copyErr := copyFile(tempPath, targetPath); copyErr != nil {
			cleanupTemp()
			restoreBackup()
			return fmt.Errorf("failed to install single-file plugin: %w", copyErr)
		}
		cleanupTemp()
	}

	if progressCallback != nil {
		progressCallback(i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_install_progress_loading"))
	}
	if loadErr := pluginManager.ReloadPlugin(ctx, metadata); loadErr != nil {
		restoreBackup()
		return fmt.Errorf("failed to load single-file plugin %s(%s): %s", metadata.GetName(ctx), metadata.Version, loadErr.Error())
	}

	if backupPath != "" {
		_ = os.Remove(backupPath)
	}
	if existingPath != "" && !pluginPathsEqual(existingPath, targetPath) && util.IsFileExists(existingPath) {
		_ = os.Remove(existingPath)
	}

	if progressCallback != nil {
		progressCallback(i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_install_progress_complete"))
	}
	logger.Info(ctx, fmt.Sprintf("single-file plugin %s(%s) installed", metadata.GetName(ctx), metadata.Version))
	return nil
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
