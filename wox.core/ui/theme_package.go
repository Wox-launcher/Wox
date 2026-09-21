package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"wox/common"
	"wox/updater"
	"wox/util"
)

// InstallPackage shares the normal install/apply/sync path after archive validation.
func (s *Store) InstallPackage(ctx context.Context, filePath string) error {
	theme, err := common.ReadThemePackage(filePath, updater.CURRENT_VERSION)
	if err != nil {
		return err
	}
	return s.Install(ctx, theme)
}

// persistThemePackage stages a complete directory and restores the old one if replacement fails.
func persistThemePackage(directory string, theme common.Theme) error {
	if err := theme.ValidateAssets(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		return err
	}
	stage, err := os.MkdirTemp(directory, ".theme-stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	for name, content := range theme.AssetFiles {
		target := filepath.Join(stage, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, content, 0644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(stage, "theme.json"), data, 0644); err != nil {
		return err
	}
	target := filepath.Join(directory, theme.ThemeId)
	backup := stage + "-previous"
	hadPrevious := false
	if info, err := os.Lstat(target); err == nil {
		// A package ID must not replace a legacy JSON file or a shared asset directory.
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("theme package destination is not a directory: %s", target)
		}
		previous, readErr := os.ReadFile(filepath.Join(target, "theme.json"))
		var identity struct{ ThemeId string }
		if readErr != nil || json.Unmarshal(previous, &identity) != nil || identity.ThemeId != theme.ThemeId {
			return fmt.Errorf("theme package destination belongs to another resource: %s", target)
		}
		if err := os.Rename(target, backup); err != nil {
			return err
		}
		hadPrevious = true
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(stage, target); err != nil {
		if hadPrevious {
			if restoreErr := os.Rename(backup, target); restoreErr != nil {
				return fmt.Errorf("install: %v; restore previous package: %w", err, restoreErr)
			}
		}
		return err
	}
	if hadPrevious {
		if err := os.RemoveAll(backup); err != nil {
			util.GetLogger().Warn(context.Background(), fmt.Sprintf("remove previous theme package: %v", err))
		}
	}
	return nil
}
