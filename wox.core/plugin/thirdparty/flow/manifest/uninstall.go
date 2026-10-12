package manifest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"wox/database"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/util"
	"wox/util/trash"
)

// Uninstall removes one plugin copied into plugins/flow-jsonrpc.
// It returns false when the instance is not one of those children, so the caller
// can keep its own uninstall path. It does not stop the shared flow host.
func Uninstall(ctx context.Context, instance *plugin.Instance, skipCleanSetting bool, preserveCache bool, progress plugin.UninstallProgressCallback) (bool, error) {
	if instance == nil {
		return false, nil
	}
	root := CollectionDirectory()
	if !flowPluginDirectoryIsChild(root, instance.PluginDirectory) {
		return false, nil
	}

	reportFlowProgress(ctx, progress, "plugin_uninstall_progress_preparing")
	reportFlowProgress(ctx, progress, "plugin_uninstall_progress_unloading")
	plugin.GetPluginManager().UnloadPlugin(ctx, instance)

	reportFlowProgress(ctx, progress, "plugin_uninstall_progress_removing")
	if err := removeFlowPluginDirectory(ctx, instance.PluginDirectory); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to remove flow plugin directory %s: %s", instance.PluginDirectory, err.Error()))
		reportFlowProgress(ctx, progress, "plugin_uninstall_progress_restoring_plugin", instance.Metadata.GetName(ctx))
		restoreFlowPlugin(ctx, instance.PluginDirectory)
		return true, err
	}

	if !skipCleanSetting {
		reportFlowProgress(ctx, progress, "plugin_uninstall_progress_cleaning_settings")
		if db := database.GetDB(); db != nil {
			if err := setting.NewPluginSettingStore(db, instance.Metadata.Id).DeleteAll(); err != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("failed to delete flow plugin settings %s: %s", instance.Metadata.Id, err.Error()))
			}
		}
	}
	if !preserveCache {
		reportFlowProgress(ctx, progress, "plugin_uninstall_progress_cleaning_cache")
		if err := util.GetLocation().RemovePluginCacheDirectory(instance.Metadata.Id); err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to delete flow plugin cache %s: %s", instance.Metadata.Id, err.Error()))
		}
	}
	reportFlowProgress(ctx, progress, "plugin_uninstall_progress_complete")
	return true, nil
}

// flowPluginDirectoryIsChild reports whether directory is exactly one safe path
// element inside root. The collection root itself is not a plugin.
func flowPluginDirectoryIsChild(root, directory string) bool {
	root = filepath.Clean(filepath.FromSlash(strings.TrimSpace(root)))
	directory = filepath.Clean(filepath.FromSlash(strings.TrimSpace(directory)))
	if root == "" || root == "." || directory == "" || directory == "." {
		return false
	}
	compareRoot, compareDirectory := root, directory
	if runtime.GOOS == "windows" {
		compareRoot = strings.ToLower(root)
		compareDirectory = strings.ToLower(directory)
	}
	relative, err := filepath.Rel(compareRoot, compareDirectory)
	if err != nil || relative == "." || relative == "" {
		return false
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	if strings.ContainsRune(relative, filepath.Separator) {
		return false
	}
	return flowDirectoryIDSafe(relative)
}

// removeFlowPluginDirectory deletes one plugin directory. Trash is preferred,
// and a failed trash move falls back to deleting the directory in place.
func removeFlowPluginDirectory(ctx context.Context, directory string) error {
	if err := trash.MoveToTrash(directory); err != nil {
		// In-place deletion covers a trash backend that cannot see this directory.
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to move flow plugin directory %s to trash: %s", directory, err.Error()))
	} else {
		return nil
	}

	delays := []time.Duration{0, 100 * time.Millisecond, 300 * time.Millisecond}
	var removeErr error
	for attempt, delay := range delays {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		removeErr = os.RemoveAll(directory)
		if removeErr == nil || os.IsNotExist(removeErr) {
			return nil
		}
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to remove flow plugin directory %s on attempt %d: %s", directory, attempt+1, removeErr.Error()))
	}
	return removeErr
}

// restoreFlowPlugin loads the directory again after a failed delete.
func restoreFlowPlugin(ctx context.Context, directory string) {
	if strings.TrimSpace(directory) == "" {
		return
	}
	descriptor, err := Parse(directory)
	if err != nil || (descriptor.Kind != KindScript && descriptor.Kind != KindDotNet) {
		return
	}
	if err := plugin.GetPluginManager().LoadRuntimePlugin(ctx, descriptor.Metadata); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to restore flow plugin %s: %s", directory, err.Error()))
	}
}

func reportFlowProgress(ctx context.Context, progress func(string), key string, args ...any) {
	if progress == nil {
		return
	}
	message := i18n.GetI18nManager().TranslateWox(ctx, "i18n:"+key)
	if len(args) > 0 {
		message = fmt.Sprintf(message, args...)
	}
	progress(message)
}

// flowDirectoryIDSafe rejects ids that would escape plugins/flow-jsonrpc/<id>.
func flowDirectoryIDSafe(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || id == "." || id == ".." {
		return false
	}
	if strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return false
	}
	return filepath.Base(id) == id
}
