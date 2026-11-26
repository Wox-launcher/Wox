package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wox/i18n"
	"wox/util"
)

const artifactStagingPrefix = ".wox-plugin-staging-"
const artifactUpgradePrefix = ".wox-plugin-upgrade-"

type artifactUpgrade struct {
	OldPath    string
	TargetPath string
	StagedPath string
	PluginID   string
	OldVersion string
}

func artifactParent(kind PluginArtifactKind) string {
	switch kind {
	case PluginArtifactScript:
		return util.GetLocation().GetUserScriptPluginsDirectory()
	case PluginArtifactSingleFile:
		return util.GetLocation().GetUserSingleFilePluginsDirectory()
	default:
		return util.GetLocation().GetPluginDirectory()
	}
}

// validateArtifactPath limits transaction recovery to direct children of plugin discovery directories.
func validateArtifactPath(path string) error {
	for _, kind := range []PluginArtifactKind{PluginArtifactPackage, PluginArtifactScript, PluginArtifactSingleFile} {
		parent := artifactParent(kind)
		if kind == PluginArtifactPackage && IsReservedUserPluginDirectory(filepath.Base(path)) {
			continue
		}
		if pluginPathsEqual(filepath.Dir(path), parent) && isPathInsideDirectory(path, parent) == nil {
			return nil
		}
	}
	return fmt.Errorf("invalid plugin artifact path %q", path)
}

// installStagedArtifact validates the complete replacement before unloading the live instance.
func (s *Store) installStagedArtifact(ctx context.Context, old *Instance, manifest StorePluginManifest, kind PluginArtifactKind, localArchive string, progress InstallProgressCallback) error {
	manager := GetPluginManager()
	tx := artifactUpgrade{PluginID: manifest.Id}
	if old != nil {
		if old.IsSystemPlugin || old.IsDevPlugin {
			return errors.New("cannot replace a development or system plugin")
		}
		oldKind := InstalledPluginArtifactKind(old)
		tx.OldPath = old.PluginDirectory
		if oldKind != PluginArtifactPackage {
			if old.Metadata.Entry == "" || filepath.Base(old.Metadata.Entry) != old.Metadata.Entry {
				return fmt.Errorf("invalid plugin entry %q", old.Metadata.Entry)
			}
			tx.OldPath = filepath.Join(artifactParent(oldKind), old.Metadata.Entry)
		}
		tx.OldVersion = old.Metadata.Version
		if err := validateArtifactPath(tx.OldPath); err != nil {
			return err
		}
	}
	parent := artifactParent(kind)
	if old == nil && kind == PluginArtifactSingleFile {
		existing, err := findInstalledSingleFilePluginPath(ctx, manifest.Id)
		if err != nil {
			return err
		}
		tx.OldPath = existing
	}
	name := artifactFileName(manifest.DownloadUrl)
	if kind == PluginArtifactPackage {
		name = manifest.Id + "@" + manifest.Version
	}
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, "/\\:") {
		return fmt.Errorf("invalid plugin artifact name %q", name)
	}
	tx.TargetPath = filepath.Join(parent, name)
	if err := validateArtifactPath(tx.TargetPath); err != nil {
		return err
	}
	if !pluginPathsEqual(tx.OldPath, tx.TargetPath) {
		if _, err := os.Lstat(tx.TargetPath); !os.IsNotExist(err) {
			if err != nil {
				return err
			}
			return fmt.Errorf("cannot install plugin: target %s already exists", tx.TargetPath)
		}
	}
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	root := filepath.Dir(util.GetLocation().GetPluginDirectory())
	staging, err := os.MkdirTemp(root, artifactStagingPrefix)
	if err != nil {
		return err
	}
	defer func() {
		if staging != "" {
			_ = os.RemoveAll(staging)
		}
	}()
	report := func(key string) {
		if progress != nil {
			progress(i18n.GetI18nManager().TranslateWox(ctx, "i18n:"+key))
		}
	}
	download := filepath.Join(staging, name)
	if kind == PluginArtifactPackage {
		download = filepath.Join(staging, "package.zip")
	}
	if localArchive == "" {
		report("plugin_install_progress_starting_download")
		if err := downloadPluginFile(ctx, manifest.DownloadUrl, download, pluginDownloadTimeout, func(downloaded, total int64) {
			if progress == nil {
				return
			}
			if total > 0 {
				progress(fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_install_progress_downloading"), float64(downloaded)/float64(total)*100))
			} else {
				progress(fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "i18n:plugin_install_progress_downloaded_bytes"), downloaded))
			}
		}); err != nil {
			return fmt.Errorf("failed to download plugin: %w", err)
		}
		report("plugin_install_progress_download_complete")
	} else {
		download = localArchive
	}
	staged := download
	var metadata Metadata
	switch kind {
	case PluginArtifactPackage:
		report("plugin_install_progress_extracting")
		staged = filepath.Join(staging, "artifact")
		if err := util.Unzip(download, staged); err != nil {
			return fmt.Errorf("failed to extract plugin: %w", err)
		}
		report("plugin_install_progress_extraction_complete")
		metadata, err = manager.ParseMetadata(ctx, staged)
	case PluginArtifactScript:
		report("plugin_install_progress_parsing")
		if err := os.Chmod(staged, 0755); err != nil {
			return err
		}
		metadata, err = manager.ParseScriptMetadata(ctx, staged)
	case PluginArtifactSingleFile:
		report("plugin_install_progress_parsing")
		metadata, err = manager.ParseSingleFilePluginMetadata(ctx, staged)
	default:
		return fmt.Errorf("unsupported plugin artifact %s", kind)
	}
	if err != nil {
		return fmt.Errorf("failed to parse staged plugin metadata: %w", err)
	}
	if err := validateArtifactMetadataMatchesManifest(metadata, manifest); err != nil {
		return err
	}
	if !IsAllSupportedOS(metadata.SupportedOS) || !supportsPluginPlatform(metadata) {
		return fmt.Errorf("plugin %s is unsupported on this OS", metadata.Id)
	}
	if err := ensureWoxVersionSupported(metadata.GetName(ctx), metadata.MinWoxVersion); err != nil {
		return err
	}
	if err := metadata.ValidateGlances(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	report("plugin_install_progress_loading")
	staging = ""
	err = s.promoteStagedArtifact(ctx, manager, tx, old, staged, kind)
	if err != nil {
		return err
	}
	report("plugin_install_progress_loaded")
	report("plugin_install_progress_complete")
	return nil
}

// promoteStagedArtifact journals the swap so shutdown during loading still leaves a recoverable old version.
func (s *Store) promoteStagedArtifact(ctx context.Context, manager *Manager, tx artifactUpgrade, old *Instance, staged string, kind PluginArtifactKind) error {
	staging := filepath.Dir(staged)
	defer func() {
		if staging != "" {
			_ = os.RemoveAll(staging)
		}
	}()
	dir, err := os.MkdirTemp(filepath.Dir(util.GetLocation().GetPluginDirectory()), artifactUpgradePrefix)
	if err != nil {
		return err
	}
	tx.StagedPath = staged
	journal, err := json.Marshal(tx)
	if err == nil {
		err = writeArtifactJournal(filepath.Join(dir, "tx.json"), journal)
	}
	if err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	oldKind := InstalledPluginArtifactKind(old)
	for _, path := range []struct {
		kind PluginArtifactKind
		path string
	}{{oldKind, tx.OldPath}, {kind, tx.TargetPath}} {
		if path.path == "" {
			continue
		}
		setArtifactWatchDeadline(&manager.lifecycle, path.kind, path.path, math.MaxInt64)
		defer func() {
			setArtifactWatchDeadline(&manager.lifecycle, path.kind, path.path, time.Now().Add(5*time.Second).UnixMilli())
		}()
	}
	backup := filepath.Join(dir, "artifact")
	if old != nil {
		manager.UnloadPlugin(ctx, old)
	}
	if tx.OldPath != "" {
		if err := os.Rename(tx.OldPath, backup); err != nil {
			_ = os.RemoveAll(dir)
			if old == nil {
				return err
			}
			return errors.Join(err, reloadArtifactAfterRollback(ctx, manager, old))
		}
	}
	moved := false
	err = os.Rename(staged, tx.TargetPath)
	if err == nil {
		moved = true
		err = loadInstalledArtifact(ctx, manager, tx.TargetPath, tx.PluginID, kind)
	}
	if err == nil {
		err = writeArtifactJournal(filepath.Join(dir, "committed"), nil)
	}
	if err == nil {
		if cleanErr := os.RemoveAll(dir); cleanErr != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to remove committed plugin upgrade: %v", cleanErr))
		}
		return nil
	}
	restoreCtx := context.WithoutCancel(ctx)
	if installed := manager.GetPluginInstanceById(tx.PluginID); installed != nil {
		manager.UnloadPlugin(restoreCtx, installed)
	}
	if moved {
		if removeErr := os.RemoveAll(tx.TargetPath); removeErr != nil {
			return errors.Join(err, removeErr)
		}
	}
	if tx.OldPath != "" {
		if restoreErr := os.Rename(backup, tx.OldPath); restoreErr != nil {
			// Startup recovery needs the staged file to distinguish a failed rename from a published target.
			staging = ""
			return errors.Join(err, restoreErr)
		}
	}
	_ = os.RemoveAll(dir)
	if old == nil {
		return err
	}
	if reloadErr := reloadArtifactAfterRollback(restoreCtx, manager, old); reloadErr != nil {
		return errors.Join(err, reloadErr)
	}
	recordArtifactRecovery(ctx, ArtifactRecoveryRollback, tx)
	return fmt.Errorf("plugin upgrade rolled back to %s: %w", tx.OldVersion, err)
}

// loadInstalledArtifact waits for initialization before the replacement can become durable.
func loadInstalledArtifact(ctx context.Context, manager *Manager, path, id string, kind PluginArtifactKind) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var err error
	if kind == PluginArtifactPackage {
		err = manager.LoadPlugin(ctx, path)
	} else {
		var metadata Metadata
		if kind == PluginArtifactScript {
			metadata, err = manager.ParseScriptMetadata(ctx, path)
		} else {
			metadata, err = manager.ParseSingleFilePluginMetadata(ctx, path)
		}
		if err == nil {
			err = manager.ReloadPlugin(ctx, metadata)
		}
	}
	if err != nil {
		return err
	}
	return waitForInstalledArtifact(ctx, manager, id)
}

// reloadArtifactAfterRollback detaches recovery from the canceled install request.
func reloadArtifactAfterRollback(ctx context.Context, manager *Manager, old *Instance) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := manager.ReloadPlugin(ctx, old.Metadata); err != nil {
		return err
	}
	return waitForInstalledArtifact(ctx, manager, old.Metadata.Id)
}

func waitForInstalledArtifact(ctx context.Context, manager *Manager, id string) error {
	installed := manager.GetPluginInstanceById(id)
	if installed == nil {
		return fmt.Errorf("installed plugin %s was not loaded", id)
	}
	return installed.WaitInit(ctx)
}

func writeArtifactJournal(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	err = errors.Join(err, f.Close())
	if err != nil {
		_ = os.Remove(path)
	}
	return err
}

// reconcileInterruptedPluginUpgrades restores uncommitted transactions before plugin discovery.
func reconcileInterruptedPluginUpgrades(ctx context.Context, root string) error {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var failures []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		switch {
		case strings.HasPrefix(entry.Name(), artifactUpgradePrefix):
			if err := reconcileArtifactUpgrade(ctx, dir); err != nil {
				failures = append(failures, err)
			}
		}
	}
	if len(failures) == 0 {
		for _, entry := range entries {
			if entry.IsDir() && strings.HasPrefix(entry.Name(), artifactStagingPrefix) {
				if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
					failures = append(failures, err)
				}
			}
		}
	}
	return errors.Join(failures...)
}

func reconcileArtifactUpgrade(ctx context.Context, dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "committed")); err == nil {
		return os.RemoveAll(dir)
	} else if !os.IsNotExist(err) {
		return err
	}
	backup := filepath.Join(dir, "artifact")
	_, backupErr := os.Lstat(backup)
	raw, err := os.ReadFile(filepath.Join(dir, "tx.json"))
	if os.IsNotExist(err) && os.IsNotExist(backupErr) {
		return os.RemoveAll(dir)
	}
	if err != nil {
		return err
	}
	var tx artifactUpgrade
	if err := json.Unmarshal(raw, &tx); err != nil {
		return err
	}
	if err := validateArtifactPath(tx.TargetPath); err != nil {
		return err
	}
	if tx.OldPath != "" {
		if err := validateArtifactPath(tx.OldPath); err != nil {
			return err
		}
		if os.IsNotExist(backupErr) {
			if _, err := os.Lstat(tx.OldPath); err != nil {
				return err
			}
			return os.RemoveAll(dir)
		}
		if backupErr != nil {
			return backupErr
		}
	}
	// A surviving staged artifact proves the rename did not publish this transaction's target.
	published := true
	if tx.StagedPath != "" {
		staging := filepath.Dir(tx.StagedPath)
		if !strings.HasPrefix(filepath.Base(staging), artifactStagingPrefix) || !pluginPathsEqual(filepath.Dir(staging), filepath.Dir(dir)) {
			return fmt.Errorf("invalid plugin transaction staging path")
		}
		if _, err := os.Lstat(tx.StagedPath); err == nil {
			published = false
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if published {
		if err := os.RemoveAll(tx.TargetPath); err != nil {
			return err
		}
	}
	if tx.OldPath != "" {
		if err := os.Rename(backup, tx.OldPath); err != nil {
			return err
		}
	}
	recordArtifactRecovery(ctx, ArtifactRecoveryInterrupted, tx)
	return os.RemoveAll(dir)
}

// setArtifactWatchDeadline prevents file watchers from acting on managed replacements.
func setArtifactWatchDeadline(lifecycle *pluginLifecycle, kind PluginArtifactKind, filePath string, deadline int64) {
	filePath, _ = filepath.Abs(filePath)
	if kind == PluginArtifactScript || kind == PluginArtifactSingleFile {
		lifecycle.artifactWatchIgnored.Store(filePath, deadline)
	}
}
