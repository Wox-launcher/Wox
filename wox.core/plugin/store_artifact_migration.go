package plugin

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"wox/util"
)

// migratePluginArtifact replaces a plugin across storage formats while retaining its settings and cache.
func (s *Store) migratePluginArtifact(ctx context.Context, old *Instance, manifest StorePluginManifest, kind PluginArtifactKind, progress InstallProgressCallback) error {
	if old.IsDevPlugin || old.IsSystemPlugin {
		return fmt.Errorf("cannot migrate the delivery form of a development or system plugin")
	}
	oldKind := InstalledPluginArtifactKind(old)
	var oldPath string
	switch oldKind {
	case PluginArtifactScript:
		if old.Metadata.Entry == "" || filepath.Base(old.Metadata.Entry) != old.Metadata.Entry {
			return fmt.Errorf("invalid script plugin entry %q", old.Metadata.Entry)
		}
		oldPath = filepath.Join(util.GetLocation().GetUserScriptPluginsDirectory(), old.Metadata.Entry)
		if err := isPathInsideDirectory(oldPath, util.GetLocation().GetUserScriptPluginsDirectory()); err != nil {
			return err
		}
	case PluginArtifactSingleFile:
		var err error
		oldPath, err = singleFilePluginPath(old.Metadata)
		if err != nil {
			return err
		}
	case PluginArtifactPackage:
		oldPath = old.PluginDirectory
		if err := isPathInsideDirectory(oldPath, util.GetLocation().GetPluginDirectory()); err != nil {
			return err
		}
		if filepath.Clean(filepath.Dir(oldPath)) != filepath.Clean(util.GetLocation().GetPluginDirectory()) {
			return fmt.Errorf("invalid package plugin directory %s", oldPath)
		}
	default:
		return fmt.Errorf("unsupported installed plugin artifact: %s", oldKind)
	}

	var target string
	switch kind {
	case PluginArtifactPackage:
		target = filepath.Join(util.GetLocation().GetPluginDirectory(), fmt.Sprintf("%s@%s", manifest.Id, manifest.Version))
		if err := isPathInsideDirectory(target, util.GetLocation().GetPluginDirectory()); err != nil {
			return err
		}
	case PluginArtifactScript, PluginArtifactSingleFile:
		name := artifactFileName(manifest.DownloadUrl)
		if name == "" || name == "." || name == "/" {
			return fmt.Errorf("plugin download URL has no file name")
		}
		directory := util.GetLocation().GetUserSingleFilePluginsDirectory()
		if kind == PluginArtifactScript {
			directory = util.GetLocation().GetUserScriptPluginsDirectory()
		}
		target = filepath.Join(directory, name)
		if err := isPathInsideDirectory(target, directory); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported plugin artifact: %s", kind)
	}
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("cannot migrate plugin: target %s already exists", target)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot check plugin target %s: %w", target, err)
	}

	manager := GetPluginManager()
	// Suppress watcher events throughout the transaction and briefly afterward, including
	// timers queued before the replacement. The installer itself owns loading and rollback.
	watchPaths := map[PluginArtifactKind]string{oldKind: oldPath, kind: target}
	for watchKind, watchPath := range watchPaths {
		setArtifactWatchDeadline(&manager.lifecycle, watchKind, watchPath, math.MaxInt64)
	}
	defer func() {
		for watchKind, watchPath := range watchPaths {
			setArtifactWatchDeadline(&manager.lifecycle, watchKind, watchPath, time.Now().Add(5*time.Second).UnixMilli())
		}
	}()

	// Keep the backup on the same filesystem and outside all plugin discovery directories.
	backupDir, err := os.MkdirTemp(filepath.Dir(util.GetLocation().GetPluginDirectory()), ".wox-plugin-upgrade-")
	if err != nil {
		return fmt.Errorf("failed to create plugin backup: %w", err)
	}
	backupPath := filepath.Join(backupDir, "artifact")
	manager.UnloadPlugin(ctx, old)
	if err := os.Rename(oldPath, backupPath); err != nil {
		_ = os.RemoveAll(backupDir)
		restoreErr := manager.ReloadPlugin(context.WithoutCancel(ctx), old.Metadata)
		return errors.Join(fmt.Errorf("failed to back up plugin: %w", err), restoreErr)
	}

	var installErr error
	switch kind {
	case PluginArtifactPackage:
		installErr = s.installNormalPlugin(ctx, manifest, progress, true)
	case PluginArtifactScript:
		installErr = s.installScriptPluginForStore(ctx, manifest, progress, true)
	case PluginArtifactSingleFile:
		installErr = s.installSingleFilePluginWithProgress(ctx, manifest, progress)
	}
	if installErr == nil {
		installed := manager.GetPluginInstanceById(manifest.Id)
		if installed == nil {
			installErr = fmt.Errorf("installed plugin %s was not loaded", manifest.Id)
		} else if err := installed.WaitInit(ctx); err != nil {
			installErr = fmt.Errorf("failed to initialize installed plugin: %w", err)
		}
	}
	if installErr == nil {
		if err := os.RemoveAll(backupDir); err != nil {
			logger.Warn(ctx, fmt.Sprintf("failed to remove plugin backup %s: %s", backupDir, err))
		}
		return nil
	}

	restoreCtx := context.WithoutCancel(ctx)
	if installed := manager.GetPluginInstanceById(manifest.Id); installed != nil {
		manager.UnloadPlugin(restoreCtx, installed)
	}
	if err := os.RemoveAll(target); err != nil {
		return errors.Join(installErr, fmt.Errorf("failed to remove incomplete plugin %s: %w; old plugin backup remains at %s", target, err, backupPath))
	}
	if err := os.Rename(backupPath, oldPath); err != nil {
		return errors.Join(installErr, fmt.Errorf("failed to restore plugin backup %s: %w", backupPath, err))
	}
	_ = os.RemoveAll(backupDir)
	if err := manager.ReloadPlugin(restoreCtx, old.Metadata); err != nil {
		return errors.Join(installErr, fmt.Errorf("failed to reload restored plugin: %w", err))
	}
	if restored := manager.GetPluginInstanceById(old.Metadata.Id); restored != nil {
		if err := restored.WaitInit(restoreCtx); err != nil {
			return errors.Join(installErr, fmt.Errorf("failed to initialize restored plugin: %w", err))
		}
	}
	return installErr
}

// setArtifactWatchDeadline prevents file watchers from acting on managed replacements.
func setArtifactWatchDeadline(lifecycle *pluginLifecycle, kind PluginArtifactKind, filePath string, deadline int64) {
	filePath, _ = filepath.Abs(filePath)
	if kind == PluginArtifactScript || kind == PluginArtifactSingleFile {
		lifecycle.artifactWatchIgnored.Store(filePath, deadline)
	}
}
