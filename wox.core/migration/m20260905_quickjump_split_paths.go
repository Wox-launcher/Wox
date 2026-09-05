package migration

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"wox/database"
	"wox/setting"
	"wox/util"

	"gorm.io/gorm"
)

const quickJumpPathsSettingKey = "quickJumpPaths"

func init() {
	Register(&quickjumpSplitPathsMigration{})
}

type quickjumpSplitPathsMigration struct{}

type quickJumpPathSetting struct {
	Path string `json:"Path"`
}

func (m *quickjumpSplitPathsMigration) ID() string {
	return "20260905_quickjump_split_paths"
}

func (m *quickjumpSplitPathsMigration) Description() string {
	return "Move leftover Quick Jump paths onto the current platform key and keep other-OS paths in the unsuffixed key for the other machine."
}

func (m *quickjumpSplitPathsMigration) Up(_ context.Context, tx *gorm.DB) error {
	platformKey := setting.PlatformSettingKey(quickJumpPathsSettingKey, util.GetCurrentPlatform())
	global, err := loadQuickJumpPluginSetting(tx, quickJumpPathsSettingKey)
	if err != nil {
		return err
	}
	platform, err := loadQuickJumpPluginSetting(tx, platformKey)
	if err != nil {
		return err
	}
	if global == nil && platform == nil {
		return nil
	}

	var globalCurrent, globalOther []string
	if global != nil {
		globalCurrent, globalOther = splitQuickJumpPathsJSON(global.Value)
	}
	var platformCurrent, platformOther []string
	if platform != nil {
		platformCurrent, platformOther = splitQuickJumpPathsJSON(platform.Value)
	}

	if platform == nil {
		isLocal := false
		if global != nil {
			isLocal = global.IsLocal
		}
		if err := saveQuickJumpPluginSetting(tx, platformKey, globalCurrent, isLocal); err != nil {
			return err
		}
	} else if len(platformOther) > 0 {
		if err := saveQuickJumpPluginSetting(tx, platformKey, platformCurrent, platform.IsLocal); err != nil {
			return err
		}
	}

	leftoverGlobal := uniqueQuickJumpPaths(append(append([]string{}, globalOther...), platformOther...))
	if len(leftoverGlobal) == 0 {
		return deleteLeftoverQuickJumpPaths(tx, global)
	}

	payload, err := marshalQuickJumpPaths(leftoverGlobal)
	if err != nil {
		return err
	}
	if global != nil && global.Value == payload {
		return nil
	}

	isLocal := false
	if global != nil {
		isLocal = global.IsLocal
	}
	if err := tx.Save(&database.PluginSetting{PluginID: quickJumpPluginID, Key: quickJumpPathsSettingKey, Value: payload, IsLocal: isLocal}).Error; err != nil {
		return err
	}
	if isLocal {
		return nil
	}
	return appendPluginSettingUpsertOplog(tx, quickJumpPluginID, quickJumpPathsSettingKey, payload)
}

func deleteLeftoverQuickJumpPaths(tx *gorm.DB, global *database.PluginSetting) error {
	if global == nil {
		return nil
	}
	if err := tx.Delete(&database.PluginSetting{PluginID: quickJumpPluginID, Key: quickJumpPathsSettingKey}).Error; err != nil {
		return err
	}
	if err := convertPluginSettingOplogsToCurrentPlatform(tx, quickJumpPluginID, quickJumpPathsSettingKey); err != nil {
		return err
	}
	return appendPluginSettingDeleteOplog(tx, quickJumpPluginID, quickJumpPathsSettingKey)
}

func loadQuickJumpPluginSetting(tx *gorm.DB, key string) (*database.PluginSetting, error) {
	var row database.PluginSetting
	err := tx.Where("plugin_id = ? AND key = ?", quickJumpPluginID, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func saveQuickJumpPluginSetting(tx *gorm.DB, key string, paths []string, isLocal bool) error {
	payload, err := marshalQuickJumpPaths(paths)
	if err != nil {
		return err
	}
	if err := tx.Save(&database.PluginSetting{PluginID: quickJumpPluginID, Key: key, Value: payload, IsLocal: isLocal}).Error; err != nil {
		return err
	}
	if isLocal {
		return nil
	}
	return appendPluginSettingUpsertOplog(tx, quickJumpPluginID, key, payload)
}

func splitQuickJumpPathsJSON(raw string) (current []string, other []string) {
	for _, path := range parseQuickJumpPathsJSON(raw) {
		if isCurrentPlatformQuickJumpPath(path) {
			current = append(current, filepath.Clean(path))
			continue
		}
		other = append(other, path)
	}
	return current, other
}

func parseQuickJumpPathsJSON(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var entries []quickJumpPathSetting
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil
	}

	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		path := strings.TrimSpace(entry.Path)
		if path == "" {
			continue
		}
		result = append(result, path)
	}
	return result
}

func marshalQuickJumpPaths(paths []string) (string, error) {
	entries := make([]quickJumpPathSetting, 0, len(paths))
	for _, path := range paths {
		entries = append(entries, quickJumpPathSetting{Path: path})
	}
	payload, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func uniqueQuickJumpPaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		if seen[path] {
			continue
		}
		seen[path] = true
		result = append(result, path)
	}
	return result
}

func isCurrentPlatformQuickJumpPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}
	if util.IsWindows() {
		return isWindowsQuickJumpPath(path)
	}
	return isUnixQuickJumpPath(path)
}

func isWindowsQuickJumpPath(path string) bool {
	if filepath.VolumeName(path) != "" {
		return true
	}
	return strings.HasPrefix(path, `\\`) || strings.HasPrefix(filepath.ToSlash(path), "//")
}

func isUnixQuickJumpPath(path string) bool {
	if filepath.VolumeName(path) != "" {
		return false
	}
	normalized := filepath.ToSlash(path)
	if strings.HasPrefix(normalized, "//") {
		return false
	}
	return strings.HasPrefix(path, "/")
}
