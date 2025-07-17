package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"wox/common"
	"wox/database"
	"wox/i18n"
	"wox/setting"
	"wox/util"
	"wox/util/locale"
)

// This file contains the logic for a one-time migration from the old JSON-based settings
// to the new SQLite database. It is designed to be self-contained.

// oldPlatformSettingValue mirrors the old PlatformSettingValue[T] generic struct.
// We define it locally to avoid dependencies on the old setting structure.
type oldPlatformSettingValue[T any] struct {
	WinValue   T `json:"WinValue"`
	MacValue   T `json:"MacValue"`
	LinuxValue T `json:"LinuxValue"`
}

func (p *oldPlatformSettingValue[T]) Get() T {
	// This is a simplified Get method for migration purposes.
	// It doesn't represent the full platform-specific logic of the original.
	// It is kept here for reference but should not be used for migration.
	// The entire object should be marshalled to JSON instead.
	if util.IsMacOS() {
		return p.MacValue
	}
	if util.IsWindows() {
		return p.WinValue
	}
	if util.IsLinux() {
		return p.LinuxValue
	}

	// Default to Mac value as a fallback
	return p.MacValue
}

// oldWoxSetting is a snapshot of the old WoxSetting struct.
type oldWoxSetting struct {
	EnableAutostart      oldPlatformSettingValue[bool]
	MainHotkey           oldPlatformSettingValue[string]
	SelectionHotkey      oldPlatformSettingValue[string]
	UsePinYin            bool
	SwitchInputMethodABC bool
	HideOnStart          bool
	HideOnLostFocus      bool
	ShowTray             bool
	LangCode             i18n.LangCode
	QueryHotkeys         oldPlatformSettingValue[[]oldQueryHotkey]
	QueryShortcuts       []oldQueryShortcut
	LastQueryMode        string
	ShowPosition         string
	AIProviders          []oldAIProvider
	EnableAutoBackup     bool
	EnableAutoUpdate     bool
	CustomPythonPath     oldPlatformSettingValue[string]
	CustomNodejsPath     oldPlatformSettingValue[string]
	HttpProxyEnabled     oldPlatformSettingValue[bool]
	HttpProxyUrl         oldPlatformSettingValue[string]
	AppWidth             int
	MaxResultCount       int
	ThemeId              string
	LastWindowX          int
	LastWindowY          int
}

// oldQueryHotkey is a snapshot of the old QueryHotkey struct.
type oldQueryHotkey struct {
	Hotkey            string
	Query             string
	IsSilentExecution bool
}

// oldQueryShortcut is a snapshot of the old QueryShortcut struct.
type oldQueryShortcut struct {
	Shortcut string
	Query    string
}

// oldAIProvider is a snapshot of the old AIProvider struct.
type oldAIProvider struct {
	Name   common.ProviderName
	ApiKey string
	Host   string
}

// oldQueryHistory is a snapshot of the old QueryHistory struct.
type oldQueryHistory struct {
	Query     common.PlainQuery
	Timestamp int64
}

// oldWoxAppData is a snapshot of the old WoxAppData struct.
type oldWoxAppData struct {
	QueryHistories  []oldQueryHistory
	FavoriteResults *util.HashMap[string, bool]
}

func getOldDefaultWoxSetting() oldWoxSetting {
	usePinYin := false
	langCode := i18n.LangCodeEnUs
	switchInputMethodABC := false
	if locale.IsZhCN() {
		usePinYin = true
		switchInputMethodABC = true
		langCode = i18n.LangCodeZhCn
	}

	return oldWoxSetting{
		MainHotkey:           oldPlatformSettingValue[string]{WinValue: "alt+space", MacValue: "command+space", LinuxValue: "ctrl+ctrl"},
		SelectionHotkey:      oldPlatformSettingValue[string]{WinValue: "win+alt+space", MacValue: "command+option+space", LinuxValue: "ctrl+shift+j"},
		UsePinYin:            usePinYin,
		SwitchInputMethodABC: switchInputMethodABC,
		ShowTray:             true,
		HideOnLostFocus:      true,
		LangCode:             langCode,
		LastQueryMode:        "empty",
		ShowPosition:         "mouse_screen",
		AppWidth:             800,
		MaxResultCount:       10,
		ThemeId:              "e4006bd3-6bfe-4020-8d1c-4c32a8e567e5",
		EnableAutostart:      oldPlatformSettingValue[bool]{WinValue: false, MacValue: false, LinuxValue: false},
		HttpProxyEnabled:     oldPlatformSettingValue[bool]{WinValue: false, MacValue: false, LinuxValue: false},
		HttpProxyUrl:         oldPlatformSettingValue[string]{WinValue: "", MacValue: "", LinuxValue: ""},
		CustomPythonPath:     oldPlatformSettingValue[string]{WinValue: "", MacValue: "", LinuxValue: ""},
		CustomNodejsPath:     oldPlatformSettingValue[string]{WinValue: "", MacValue: "", LinuxValue: ""},
		EnableAutoBackup:     true,
		EnableAutoUpdate:     true,
		LastWindowX:          -1,
		LastWindowY:          -1,
	}
}

func Run(ctx context.Context) error {
	// if database exists, no need to migrate
	if _, err := os.Stat(util.GetLocation().GetUserDataDirectory() + "wox.db"); err == nil {
		util.GetLogger().Info(ctx, "database found, skip for migrate.")
		return nil
	}

	util.GetLogger().Info(ctx, "database not found. Checking for old configuration files to migrate.")

	oldSettingPath := util.GetLocation().GetWoxSettingPath()
	oldAppDataPath := util.GetLocation().GetWoxAppDataPath()

	_, settingStatErr := os.Stat(oldSettingPath)
	_, appDataStatErr := os.Stat(oldAppDataPath)

	if os.IsNotExist(settingStatErr) && os.IsNotExist(appDataStatErr) {
		util.GetLogger().Info(ctx, "no old configuration files found. Skipping migration.")
		return nil
	}

	util.GetLogger().Info(ctx, "old configuration files found. Starting migration process.")

	migrateDB := database.GetDB()

	// Load old settings
	oldSettings := getOldDefaultWoxSetting()
	if _, err := os.Stat(oldSettingPath); err == nil {
		fileContent, readErr := os.ReadFile(oldSettingPath)
		if readErr == nil && len(fileContent) > 0 {
			if unmarshalErr := json.Unmarshal(fileContent, &oldSettings); unmarshalErr != nil {
				util.GetLogger().Warn(ctx, fmt.Sprintf("failed to unmarshal old wox.setting.json: %v, will use defaults for migration.", unmarshalErr))
			} else {
				util.GetLogger().Info(ctx, "successfully loaded old wox.setting.json for migration.")
			}
		}
	}

	// Load old app data
	var oldAppData oldWoxAppData
	if oldAppData.QueryHistories == nil {
		oldAppData.QueryHistories = []oldQueryHistory{}
	}
	if _, err := os.Stat(oldAppDataPath); err == nil {
		fileContent, readErr := os.ReadFile(oldAppDataPath)
		if readErr == nil && len(fileContent) > 0 {
			if json.Unmarshal(fileContent, &oldAppData) != nil {
				util.GetLogger().Warn(ctx, "failed to unmarshal old wox.app.data.json, will use defaults for migration.")
			}
		}
	}

	tx := migrateDB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		} else if err := tx.Error; err != nil {
			tx.Rollback()
		}
	}()

	store := setting.NewStore(tx)

	// Migrate simple settings
	settingsToMigrate := map[string]interface{}{
		"EnableAutostart":      oldSettings.EnableAutostart,
		"MainHotkey":           oldSettings.MainHotkey,
		"SelectionHotkey":      oldSettings.SelectionHotkey,
		"UsePinYin":            oldSettings.UsePinYin,
		"SwitchInputMethodABC": oldSettings.SwitchInputMethodABC,
		"HideOnStart":          oldSettings.HideOnStart,
		"HideOnLostFocus":      oldSettings.HideOnLostFocus,
		"ShowTray":             oldSettings.ShowTray,
		"LangCode":             oldSettings.LangCode,
		"LastQueryMode":        oldSettings.LastQueryMode,
		"ShowPosition":         oldSettings.ShowPosition,
		"EnableAutoBackup":     oldSettings.EnableAutoBackup,
		"EnableAutoUpdate":     oldSettings.EnableAutoUpdate,
		"CustomPythonPath":     oldSettings.CustomPythonPath,
		"CustomNodejsPath":     oldSettings.CustomNodejsPath,
		"HttpProxyEnabled":     oldSettings.HttpProxyEnabled,
		"HttpProxyUrl":         oldSettings.HttpProxyUrl,
		"AppWidth":             oldSettings.AppWidth,
		"MaxResultCount":       oldSettings.MaxResultCount,
		"ThemeId":              oldSettings.ThemeId,
		"LastWindowX":          oldSettings.LastWindowX,
		"LastWindowY":          oldSettings.LastWindowY,

		"QueryHotkeys":   oldSettings.QueryHotkeys,
		"QueryShortcuts": oldSettings.QueryShortcuts,
		"AIProviders":    oldSettings.AIProviders,
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("migrating %d core settings", len(settingsToMigrate)))
	for key, value := range settingsToMigrate {
		util.GetLogger().Info(ctx, fmt.Sprintf("migrating setting %s", key))
		if err := store.Set(key, value); err != nil {
			return fmt.Errorf("failed to migrate setting %s: %w", key, err)
		}
	}

	// Migrate plugin settings
	pluginDir := util.GetLocation().GetPluginSettingDirectory()
	dirs, err := os.ReadDir(pluginDir)
	if err == nil {
		for _, file := range dirs {
			if file.IsDir() {
				continue
			}
			if !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			if strings.Contains(file.Name(), "wox") {
				continue
			}

			pluginId := strings.TrimSuffix(file.Name(), ".json")
			pluginJsonPath := path.Join(pluginDir, file.Name())
			if _, err := os.Stat(pluginJsonPath); err != nil {
				continue
			}

			content, err := os.ReadFile(pluginJsonPath)
			if err != nil {
				continue
			}
			var setting struct {
				Name     string            `json:"Name"`
				Settings map[string]string `json:"Settings"`
			}
			if err := json.Unmarshal(content, &setting); err != nil {
				continue
			}
			util.GetLogger().Info(ctx, fmt.Sprintf("migrating plugin settings for %s (%s)", setting.Name, pluginId))

			counter := 0
			for key, value := range setting.Settings {
				if value == "" {
					continue
				}
				if err := store.SetPluginSetting(pluginId, key, value); err != nil {
					util.GetLogger().Warn(ctx, fmt.Sprintf("failed to migrate plugin setting %s for %s: %v", key, pluginId, err))
					continue
				}
				counter++
			}
			if err := os.Rename(pluginJsonPath, pluginJsonPath+".bak"); err != nil {
				util.GetLogger().Warn(ctx, fmt.Sprintf("failed to rename old plugin setting file to .bak for %s: %v", pluginId, err))
			}

			util.GetLogger().Info(ctx, fmt.Sprintf("migrated %d plugin settings for %s", counter, setting.Name))
		}
	}

	// Migrate query history
	if len(oldAppData.QueryHistories) > 0 {
		util.GetLogger().Info(ctx, fmt.Sprintf("migrating %d query histories", len(oldAppData.QueryHistories)))
		if err := store.Set("QueryHistories", oldAppData.QueryHistories); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to migrate query histories: %v", err))
		}
	}

	// Migrate favorite results
	if oldAppData.FavoriteResults != nil {
		util.GetLogger().Info(ctx, fmt.Sprintf("migrating %d favorite results", oldAppData.FavoriteResults.Len()))
		if err := store.Set("FavoriteResults", oldAppData.FavoriteResults); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to migrate favorite results: %v", err))
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit migration transaction: %w", err)
	}

	if _, err := os.Stat(oldSettingPath); err == nil {
		if err := os.Rename(oldSettingPath, oldSettingPath+".bak"); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("Failed to rename old setting file to .bak: %v", err))
		}
	}
	if _, err := os.Stat(oldAppDataPath); err == nil {
		if err := os.Rename(oldAppDataPath, oldAppDataPath+".bak"); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("Failed to rename old app data file to .bak: %v", err))
		}
	}

	util.GetLogger().Info(ctx, "Successfully migrated old configuration to the new database.")
	return nil
}
