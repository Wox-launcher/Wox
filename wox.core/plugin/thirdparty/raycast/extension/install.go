package extension

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"wox/database"
	"wox/plugin"
	"wox/setting"
	"wox/util"
)

type Layer struct{}

// File replacement and removal share a lock now that imports do not go through Store's install lock.
var installationMu sync.Mutex

func (Layer) Name() string          { return "raycast" }
func (Layer) Directory() string     { return DirectoryName }
func (Layer) SupportedOS() []string { return nil }
func (Layer) Hosts() []plugin.Host  { return []plugin.Host{&Host{}} }

// Import stages a complete copy before replacing an installed extension.
// Dependencies must already be built: importing never runs package scripts or a compiler.
func Import(ctx context.Context, source string) error {
	installationMu.Lock()
	defer installationMu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	d, err := Parse(source)
	if err != nil {
		return err
	}
	if !plugin.IsAnySupportedInCurrentOS(d.Metadata.SupportedOS) && len(d.Metadata.SupportedOS) != 0 {
		return fmt.Errorf("Raycast extension does not support this platform")
	}
	if err := plugin.GetPluginManager().EnsureHostStarted(ctx, Runtime); err != nil {
		return err
	}
	root := CollectionDirectory()
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	if err := validateImportSource(d.Metadata.Directory, root); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(root, ".import-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := copyExtension(ctx, d.Metadata.Directory, stage); err != nil {
		return err
	}
	staged, err := Parse(stage)
	if err != nil {
		return err
	}
	if staged.Metadata.Id != d.Metadata.Id {
		return fmt.Errorf("Raycast manifest changed during import")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	dest := filepath.Join(root, d.Metadata.Id)
	var existing *plugin.Instance
	for _, instance := range plugin.GetPluginManager().GetPluginInstances() {
		if instance.Metadata.Id == d.Metadata.Id {
			if instance.Metadata.Runtime != string(Runtime) {
				return fmt.Errorf("plugin ID is already owned by another runtime")
			}
			existing = instance
			break
		}
	}
	if existing != nil {
		plugin.GetPluginManager().UnloadPlugin(ctx, existing)
	}
	err = replaceDirectory(stage, dest, func() error {
		installed, err := Parse(dest)
		if err != nil {
			return err
		}
		return plugin.GetPluginManager().LoadRuntimePlugin(ctx, installed.Metadata)
	}, func() error {
		if existing != nil {
			return plugin.GetPluginManager().LoadRuntimePlugin(ctx, existing.Metadata)
		}
		return nil
	}, func(err error) { util.GetLogger().Warn(ctx, fmt.Sprintf("Raycast previous import cleanup: %v", err)) })
	if err != nil {
		return err
	}
	if ui := plugin.GetPluginManager().GetUI(); ui != nil {
		ui.ReloadSettingPlugins(ctx)
	}
	return nil
}

// validateImportSource rejects overlap in either direction before creating a staging directory.
// Resolving aliases prevents an import from recursively copying its own staging files.
func validateImportSource(source, collection string) error {
	source, err := filepath.EvalSymlinks(source)
	if err != nil {
		return err
	}
	collection, err = filepath.EvalSymlinks(collection)
	if err != nil {
		return err
	}
	for _, pair := range [][2]string{{source, collection}, {collection, source}} {
		rel, err := filepath.Rel(pair[0], pair[1])
		// Different volumes cannot overlap and are valid import sources.
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("choose an extension directory outside the Wox plugin collection")
		}
	}
	return nil
}

// replaceDirectory restores both files and the loaded instance if activation fails.
// A failed filesystem rollback retains the backup and reports its path for recovery.
func replaceDirectory(stage, dest string, activate, restore func() error, cleanupWarning func(error)) error {
	backup := stage + "-previous"
	hadPrevious := false
	if _, err := os.Lstat(dest); err == nil {
		if err := os.Rename(dest, backup); err != nil {
			return errors.Join(err, restore())
		}
		hadPrevious = true
	} else if !os.IsNotExist(err) {
		return errors.Join(err, restore())
	}
	rollback := func(cause error, installed bool) error {
		if installed {
			if err := os.RemoveAll(dest); err != nil {
				return errors.Join(cause, fmt.Errorf("rollback failed; previous files retained at %s: %w", backup, err))
			}
		}
		if hadPrevious {
			if err := os.Rename(backup, dest); err != nil {
				return errors.Join(cause, fmt.Errorf("restore %s: %w", backup, err))
			}
		}
		return errors.Join(cause, restore())
	}
	if err := os.Rename(stage, dest); err != nil {
		return rollback(err, false)
	}
	if err := activate(); err != nil {
		return rollback(err, true)
	}
	if hadPrevious {
		if err := os.RemoveAll(backup); err != nil {
			cleanupWarning(err)
		}
	}
	return nil
}

// copyExtension rejects links and bounds disk usage while preserving native helper execute bits.
func copyExtension(ctx context.Context, source, dest string) error {
	var total int64
	count := 0
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == ".github") {
			return filepath.SkipDir
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
			return fmt.Errorf("unsupported linked or special extension file: %s", rel)
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		count++
		total += info.Size()
		if count > 20000 || total > 256<<20 {
			return fmt.Errorf("Raycast extension exceeds import limits")
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			return err
		}
		copied, copyErr := io.Copy(out, io.LimitReader(in, info.Size()+1))
		closeErr := out.Close()
		if copyErr != nil {
			return copyErr
		}
		if copied != info.Size() {
			return fmt.Errorf("extension file changed during import: %s", rel)
		}
		return closeErr
	})
}

// Uninstall owns removal of the extension collection and its Wox settings/cache.
func (Layer) Uninstall(ctx context.Context, instance *plugin.Instance, skipSettings, preserveCache bool, progress plugin.UninstallProgressCallback) (bool, error) {
	if instance == nil || instance.Metadata.Runtime != string(Runtime) {
		return false, nil
	}
	installationMu.Lock()
	defer installationMu.Unlock()
	root := filepath.Clean(CollectionDirectory())
	directory := filepath.Clean(instance.PluginDirectory)
	if !strings.EqualFold(filepath.Dir(directory), root) || filepath.Base(directory) != instance.Metadata.Id {
		return true, fmt.Errorf("invalid Raycast plugin directory")
	}
	plugin.GetPluginManager().UnloadPlugin(ctx, instance)
	if err := os.RemoveAll(directory); err != nil {
		_ = plugin.GetPluginManager().LoadRuntimePlugin(ctx, instance.Metadata)
		return true, err
	}
	if !skipSettings && database.GetDB() != nil {
		if err := setting.NewPluginSettingStore(database.GetDB(), instance.Metadata.Id).DeleteAll(); err != nil {
			return true, err
		}
	}
	if !preserveCache {
		return true, util.GetLocation().RemovePluginCacheDirectory(instance.Metadata.Id)
	}
	return true, nil
}
