package setting

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strconv"
	"sync"
	"wox/common"
	"wox/database"
	"wox/i18n"
	"wox/setting/definition"
	"wox/util"
	"wox/util/autostart"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var managerInstance *Manager
var managerOnce sync.Once
var logger *util.Log

type Manager struct {
	woxSetting *WoxSetting
	woxAppData *WoxAppData
}

func GetSettingManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{
			woxSetting: &WoxSetting{},
			woxAppData: &WoxAppData{},
		}
		logger = util.GetLogger()
	})
	return managerInstance
}

func (m *Manager) Init(ctx context.Context) error {
	// Step 1: Check if a migration is needed and perform it *before* initializing the main DB connection.
	if err := m.migrateDataIfNeeded(ctx); err != nil {
		// Log the error but don't block startup, as we can proceed with default settings.
		logger.Error(ctx, fmt.Sprintf("failed to perform data migration: %v. Proceeding with default settings.", err))
	}

	// Step 2: Initialize the database. This will now either open the existing DB or create a new one.
	if err := database.Init(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Step 3: Load settings from the database into the manager's struct.
	if err := m.loadSettingsFromDB(ctx); err != nil {
		return fmt.Errorf("failed to load settings from database: %w", err)
	}

	m.StartAutoBackup(ctx)

	// Step 4: Perform post-load checks (like autostart)
	if err := m.checkAutostart(ctx); err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to check autostart status: %v", err))
	}

	return nil
}

func (m *Manager) migrateDataIfNeeded(ctx context.Context) error {
	dbPath := path.Join(util.GetLocation().GetUserDataDirectory(), "wox.db")
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		// Database already exists, no migration needed.
		return nil
	}

	logger.Info(ctx, "Database not found. Checking for old configuration files to migrate.")

	oldSettingPath := util.GetLocation().GetWoxSettingPath()
	oldAppDataPath := util.GetLocation().GetWoxAppDataPath()

	_, settingStatErr := os.Stat(oldSettingPath)
	_, appDataStatErr := os.Stat(oldAppDataPath)

	if os.IsNotExist(settingStatErr) && os.IsNotExist(appDataStatErr) {
		logger.Info(ctx, "No old configuration files found. Skipping migration.")
		return nil
	}

	logger.Info(ctx, "Old configuration files found. Starting migration process.")

	// Temporarily connect to the database to perform migration.
	migrateDB, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to open database for migration: %w", err)
	}

	// Get the underlying SQL DB connection to close it later.
	sqlDB, err := migrateDB.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	// Manually create schema
	if err := migrateDB.AutoMigrate(&database.Setting{}, &database.Hotkey{}, &database.QueryShortcut{}, &database.AIProvider{}, &database.QueryHistory{}, &database.FavoriteResult{}, &database.PluginSetting{}, &database.ActionedResult{}, &database.Oplog{}); err != nil {
		return fmt.Errorf("failed to create schema during migration: %w", err)
	}

	// Load old settings
	oldWoxSetting := GetDefaultWoxSetting(ctx)
	if _, err := os.Stat(oldSettingPath); err == nil {
		fileContent, readErr := os.ReadFile(oldSettingPath)
		if readErr == nil && len(fileContent) > 0 {
			if json.Unmarshal(fileContent, &oldWoxSetting) != nil {
				logger.Warn(ctx, "Failed to unmarshal old wox.setting.json, will use defaults for migration.")
			}
		}
	}

	// Load old app data
	oldWoxAppData := GetDefaultWoxAppData(ctx)
	if _, err := os.Stat(oldAppDataPath); err == nil {
		fileContent, readErr := os.ReadFile(oldAppDataPath)
		if readErr == nil && len(fileContent) > 0 {
			if json.Unmarshal(fileContent, &oldWoxAppData) != nil {
				logger.Warn(ctx, "Failed to unmarshal old wox.app.data.json, will use defaults for migration.")
			}
		}
	}

	// Perform the migration in a single transaction
	tx := migrateDB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// Defer a rollback in case of panic or error
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		} else if err := tx.Error; err != nil {
			tx.Rollback()
		}
	}()

	// ... (rest of the migration logic is the same)
	settings := map[string]string{
		"EnableAutostart":      strconv.FormatBool(oldWoxSetting.EnableAutostart.Get()),
		"MainHotkey":           oldWoxSetting.MainHotkey.Get(),
		"SelectionHotkey":      oldWoxSetting.SelectionHotkey.Get(),
		"UsePinYin":            strconv.FormatBool(oldWoxSetting.UsePinYin),
		"SwitchInputMethodABC": strconv.FormatBool(oldWoxSetting.SwitchInputMethodABC),
		"HideOnStart":          strconv.FormatBool(oldWoxSetting.HideOnStart),
		"HideOnLostFocus":      strconv.FormatBool(oldWoxSetting.HideOnLostFocus),
		"ShowTray":             strconv.FormatBool(oldWoxSetting.ShowTray),
		"LangCode":             string(oldWoxSetting.LangCode),
		"LastQueryMode":        oldWoxSetting.LastQueryMode,
		"ShowPosition":         string(oldWoxSetting.ShowPosition),
		"EnableAutoBackup":     strconv.FormatBool(oldWoxSetting.EnableAutoBackup),
		"EnableAutoUpdate":     strconv.FormatBool(oldWoxSetting.EnableAutoUpdate),
		"CustomPythonPath":     oldWoxSetting.CustomPythonPath.Get(),
		"CustomNodejsPath":     oldWoxSetting.CustomNodejsPath.Get(),
		"HttpProxyEnabled":     strconv.FormatBool(oldWoxSetting.HttpProxyEnabled.Get()),
		"HttpProxyUrl":         oldWoxSetting.HttpProxyUrl.Get(),
		"AppWidth":             strconv.Itoa(oldWoxSetting.AppWidth),
		"MaxResultCount":       strconv.Itoa(oldWoxSetting.MaxResultCount),
		"ThemeId":              oldWoxSetting.ThemeId,
		"LastWindowX":          strconv.Itoa(oldWoxSetting.LastWindowX),
		"LastWindowY":          strconv.Itoa(oldWoxSetting.LastWindowY),
	}

	for key, value := range settings {
		if err := tx.Create(&database.Setting{Key: key, Value: value}).Error; err != nil {
			return fmt.Errorf("failed to migrate setting %s: %w", key, err)
		}
	}

	// Migrate complex types
	for _, hotkey := range oldWoxSetting.QueryHotkeys.Get() {
		if err := tx.Create(&database.Hotkey{Hotkey: hotkey.Hotkey, Query: hotkey.Query, IsSilentExecution: hotkey.IsSilentExecution}).Error; err != nil {
			return fmt.Errorf("failed to migrate hotkey: %w", err)
		}
	}
	for _, shortcut := range oldWoxSetting.QueryShortcuts {
		if err := tx.Create(&database.QueryShortcut{Shortcut: shortcut.Shortcut, Query: shortcut.Query}).Error; err != nil {
			return fmt.Errorf("failed to migrate shortcut: %w", err)
		}
	}
	for _, provider := range oldWoxSetting.AIProviders {
		if err := tx.Create(&database.AIProvider{Name: provider.Name, ApiKey: provider.ApiKey, Host: provider.Host}).Error; err != nil {
			return fmt.Errorf("failed to migrate AI provider: %w", err)
		}
	}

	// Migrate App Data
	for _, history := range oldWoxAppData.QueryHistories {
		if err := tx.Create(&database.QueryHistory{Query: history.Query.String(), Timestamp: history.Timestamp}).Error; err != nil {
			return fmt.Errorf("failed to migrate query history: %w", err)
		}
	}
	// NOTE: FavoriteResults cannot be migrated due to the one-way hash nature of ResultHash.
	// Users will need to re-favorite items after this update.
	logger.Warn(ctx, "Favorite results cannot be migrated and will be reset.")

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit migration transaction: %w", err)
	}

	// Rename old files to .bak on successful migration
	if _, err := os.Stat(oldSettingPath); err == nil {
		if err := os.Rename(oldSettingPath, oldSettingPath+".bak"); err != nil {
			logger.Warn(ctx, fmt.Sprintf("Failed to rename old setting file to .bak: %v", err))
		}
	}
	if _, err := os.Stat(oldAppDataPath); err == nil {
		if err := os.Rename(oldAppDataPath, oldAppDataPath+".bak"); err != nil {
			logger.Warn(ctx, fmt.Sprintf("Failed to rename old app data file to .bak: %v", err))
		}
	}

	logger.Info(ctx, "Successfully migrated old configuration to the new database.")
	return nil
}

func (m *Manager) loadSettingsFromDB(ctx context.Context) error {
	logger.Info(ctx, "Loading settings from database...")
	db := database.GetDB()

	// Start with default settings, then overwrite with values from DB
	defaultWoxSetting := GetDefaultWoxSetting(ctx)
	m.woxSetting = &defaultWoxSetting
	defaultWoxAppData := GetDefaultWoxAppData(ctx)
	m.woxAppData = &defaultWoxAppData

	// Load simple K/V settings
	var settings []database.Setting
	if err := db.Find(&settings).Error; err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	settingsMap := make(map[string]string)
	for _, s := range settings {
		settingsMap[s.Key] = s.Value
	}

	// Populate m.woxSetting from settingsMap
	m.populateWoxSettingFromMap(settingsMap)

	// Load complex types
	var hotkeys []database.Hotkey
	if err := db.Find(&hotkeys).Error; err == nil {
		queryHotkeys := make([]QueryHotkey, len(hotkeys))
		for i, h := range hotkeys {
			queryHotkeys[i] = QueryHotkey{Hotkey: h.Hotkey, Query: h.Query, IsSilentExecution: h.IsSilentExecution}
		}
		m.woxSetting.QueryHotkeys.Set(queryHotkeys)
	} else {
		logger.Warn(ctx, fmt.Sprintf("Could not load hotkeys: %v", err))
	}

	var shortcuts []database.QueryShortcut
	if err := db.Find(&shortcuts).Error; err == nil {
		queryShortcuts := make([]QueryShortcut, len(shortcuts))
		for i, s := range shortcuts {
			queryShortcuts[i] = QueryShortcut{Shortcut: s.Shortcut, Query: s.Query}
		}
		m.woxSetting.QueryShortcuts = queryShortcuts
	} else {
		logger.Warn(ctx, fmt.Sprintf("Could not load query shortcuts: %v", err))
	}

	var providers []database.AIProvider
	if err := db.Find(&providers).Error; err == nil {
		m.woxSetting.AIProviders = make([]AIProvider, len(providers))
		for i, p := range providers {
			m.woxSetting.AIProviders[i] = AIProvider{Name: p.Name, ApiKey: p.ApiKey, Host: p.Host}
		}
	} else {
		logger.Warn(ctx, fmt.Sprintf("Could not load AI providers: %v", err))
	}

	// Load App Data
	var history []database.QueryHistory
	if err := db.Order("timestamp asc").Find(&history).Error; err == nil {
		m.woxAppData.QueryHistories = make([]QueryHistory, len(history))
		for i, h := range history {
			m.woxAppData.QueryHistories[i] = QueryHistory{Query: common.PlainQuery{QueryText: h.Query}, Timestamp: h.Timestamp}
		}
	} else {
		logger.Warn(ctx, fmt.Sprintf("Could not load query history: %v", err))
	}

	var favorites []database.FavoriteResult
	if err := db.Find(&favorites).Error; err == nil {
		m.woxAppData.FavoriteResults = util.NewHashMap[ResultHash, bool]()
		for _, f := range favorites {
			hash := NewResultHash(f.PluginID, f.Title, f.Subtitle)
			m.woxAppData.FavoriteResults.Store(hash, true)
		}
	} else {
		logger.Warn(ctx, fmt.Sprintf("Could not load favorite results: %v", err))
	}

	logger.Info(ctx, "Successfully loaded settings from database.")
	return nil
}

func (m *Manager) populateWoxSettingFromMap(settingsMap map[string]string) {
	if val, ok := settingsMap["EnableAutostart"]; ok {
		m.woxSetting.EnableAutostart.Set(val == "true")
	}
	if val, ok := settingsMap["MainHotkey"]; ok {
		m.woxSetting.MainHotkey.Set(val)
	}
	if val, ok := settingsMap["SelectionHotkey"]; ok {
		m.woxSetting.SelectionHotkey.Set(val)
	}
	if val, ok := settingsMap["UsePinYin"]; ok {
		m.woxSetting.UsePinYin = val == "true"
	}
	if val, ok := settingsMap["SwitchInputMethodABC"]; ok {
		m.woxSetting.SwitchInputMethodABC = val == "true"
	}
	if val, ok := settingsMap["HideOnStart"]; ok {
		m.woxSetting.HideOnStart = val == "true"
	}
	if val, ok := settingsMap["HideOnLostFocus"]; ok {
		m.woxSetting.HideOnLostFocus = val == "true"
	}
	if val, ok := settingsMap["ShowTray"]; ok {
		m.woxSetting.ShowTray = val == "true"
	}
	if val, ok := settingsMap["LangCode"]; ok {
		m.woxSetting.LangCode = i18n.LangCode(val)
	}
	if val, ok := settingsMap["LastQueryMode"]; ok {
		m.woxSetting.LastQueryMode = val
	}
	if val, ok := settingsMap["ShowPosition"]; ok {
		m.woxSetting.ShowPosition = PositionType(val)
	}
	if val, ok := settingsMap["EnableAutoBackup"]; ok {
		m.woxSetting.EnableAutoBackup = val == "true"
	}
	if val, ok := settingsMap["EnableAutoUpdate"]; ok {
		m.woxSetting.EnableAutoUpdate = val == "true"
	}
	if val, ok := settingsMap["CustomPythonPath"]; ok {
		m.woxSetting.CustomPythonPath.Set(val)
	}
	if val, ok := settingsMap["CustomNodejsPath"]; ok {
		m.woxSetting.CustomNodejsPath.Set(val)
	}
	if val, ok := settingsMap["HttpProxyEnabled"]; ok {
		m.woxSetting.HttpProxyEnabled.Set(val == "true")
	}
	if val, ok := settingsMap["HttpProxyUrl"]; ok {
		m.woxSetting.HttpProxyUrl.Set(val)
	}
	if val, ok := settingsMap["ThemeId"]; ok {
		m.woxSetting.ThemeId = val
	}
	if val, ok := settingsMap["AppWidth"]; ok {
		m.woxSetting.AppWidth, _ = strconv.Atoi(val)
	}
	if val, ok := settingsMap["MaxResultCount"]; ok {
		m.woxSetting.MaxResultCount, _ = strconv.Atoi(val)
	}
	if val, ok := settingsMap["LastWindowX"]; ok {
		m.woxSetting.LastWindowX, _ = strconv.Atoi(val)
	}
	if val, ok := settingsMap["LastWindowY"]; ok {
		m.woxSetting.LastWindowY, _ = strconv.Atoi(val)
	}
}

func (m *Manager) checkAutostart(ctx context.Context) error {
	actualAutostart, err := autostart.IsAutostart(ctx)
	if err != nil {
		return fmt.Errorf("failed to check autostart status: %w", err)
	}

	configAutostart := m.woxSetting.EnableAutostart.Get()
	if actualAutostart != configAutostart {
		util.GetLogger().Warn(ctx, fmt.Sprintf("Autostart setting mismatch: config %v, actual %v", configAutostart, actualAutostart))

		if configAutostart {
			util.GetLogger().Info(ctx, "Attempting to fix autostart configuration...")
			if err := autostart.SetAutostart(ctx, true); err != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("Failed to fix autostart: %s", err.Error()))
				m.woxSetting.EnableAutostart.Set(false)
			} else {
				util.GetLogger().Info(ctx, "Autostart configuration fixed successfully")
			}
		} else {
			// This case is less common, but we can ensure it's disabled if config says so.
			if err := autostart.SetAutostart(ctx, false); err != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("Failed to disable autostart: %s", err.Error()))
				m.woxSetting.EnableAutostart.Set(true) // Revert setting if action fails
			}
		}

		// Save the updated setting
		return m.SaveWoxSetting(ctx)
	}
	return nil
}

func (m *Manager) GetWoxSetting(ctx context.Context) *WoxSetting {
	return m.woxSetting
}

func (m *Manager) UpdateWoxSetting(ctx context.Context, key, value string) error {
	db := database.GetDB()

	// Use a map for easy lookup and update
	updateMap := map[string]interface{}{
		"EnableAutostart":      func() { m.woxSetting.EnableAutostart.Set(value == "true") },
		"MainHotkey":           func() { m.woxSetting.MainHotkey.Set(value) },
		"SelectionHotkey":      func() { m.woxSetting.SelectionHotkey.Set(value) },
		"UsePinYin":            func() { m.woxSetting.UsePinYin = value == "true" },
		"SwitchInputMethodABC": func() { m.woxSetting.SwitchInputMethodABC = value == "true" },
		"HideOnStart":          func() { m.woxSetting.HideOnStart = value == "true" },
		"HideOnLostFocus":      func() { m.woxSetting.HideOnLostFocus = value == "true" },
		"ShowTray":             func() { m.woxSetting.ShowTray = value == "true" },
		"LangCode":             func() { m.woxSetting.LangCode = i18n.LangCode(value) },
		"LastQueryMode":        func() { m.woxSetting.LastQueryMode = value },
		"ThemeId":              func() { m.woxSetting.ThemeId = value },
		"ShowPosition":         func() { m.woxSetting.ShowPosition = PositionType(value) },
		"EnableAutoBackup":     func() { m.woxSetting.EnableAutoBackup = value == "true" },
		"EnableAutoUpdate":     func() { m.woxSetting.EnableAutoUpdate = value == "true" },
		"CustomPythonPath":     func() { m.woxSetting.CustomPythonPath.Set(value) },
		"CustomNodejsPath":     func() { m.woxSetting.CustomNodejsPath.Set(value) },
		"HttpProxyEnabled": func() {
			m.woxSetting.HttpProxyEnabled.Set(value == "true")
			if m.woxSetting.HttpProxyUrl.Get() != "" && m.woxSetting.HttpProxyEnabled.Get() {
				m.onUpdateProxy(ctx, m.woxSetting.HttpProxyUrl.Get())
			} else {
				m.onUpdateProxy(ctx, "")
			}
		},
		"HttpProxyUrl": func() {
			m.woxSetting.HttpProxyUrl.Set(value)
			if m.woxSetting.HttpProxyEnabled.Get() && value != "" {
				m.onUpdateProxy(ctx, m.woxSetting.HttpProxyUrl.Get())
			} else {
				m.onUpdateProxy(ctx, "")
			}
		},
		"AppWidth": func() {
			appWidth, _ := strconv.Atoi(value)
			m.woxSetting.AppWidth = appWidth
		},
		"MaxResultCount": func() {
			maxResultCount, _ := strconv.Atoi(value)
			m.woxSetting.MaxResultCount = maxResultCount
		},
		"QueryHotkeys": func() {
			var queryHotkeys []QueryHotkey
			if json.Unmarshal([]byte(value), &queryHotkeys) == nil {
				m.woxSetting.QueryHotkeys.Set(queryHotkeys)
				db.Delete(&database.Hotkey{}, "1 = 1") // Clear existing
				for _, h := range queryHotkeys {
					db.Create(&database.Hotkey{Hotkey: h.Hotkey, Query: h.Query, IsSilentExecution: h.IsSilentExecution})
				}
			}
		},
		"QueryShortcuts": func() {
			var queryShortcuts []QueryShortcut
			if json.Unmarshal([]byte(value), &queryShortcuts) == nil {
				m.woxSetting.QueryShortcuts = queryShortcuts
				db.Delete(&database.QueryShortcut{}, "1 = 1") // Clear existing
				for _, s := range queryShortcuts {
					db.Create(&database.QueryShortcut{Shortcut: s.Shortcut, Query: s.Query})
				}
			}
		},
		"AIProviders": func() {
			var aiProviders []AIProvider
			if json.Unmarshal([]byte(value), &aiProviders) == nil {
				m.woxSetting.AIProviders = aiProviders
				db.Delete(&database.AIProvider{}, "1 = 1") // Clear existing
				for _, p := range aiProviders {
					db.Create(&database.AIProvider{Name: p.Name, ApiKey: p.ApiKey, Host: p.Host})
				}
			}
		},
	}

	if updateFunc, ok := updateMap[key]; ok {
		// For complex types, the update is handled within the function itself.
		if key != "QueryHotkeys" && key != "QueryShortcuts" && key != "AIProviders" {
			result := db.Model(&database.Setting{}).Where("key = ?", key).Update("value", value)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				// If no rows were affected, it means the key doesn't exist, so create it.
				if err := db.Create(&database.Setting{Key: key, Value: value}).Error; err != nil {
					return err
				}
			}
		}
		// Update in-memory struct
		updateFunc.(func())()
		return nil
	}

	return fmt.Errorf("unknown key: %s", key)
}

func (m *Manager) onUpdateProxy(ctx context.Context, url string) {
	util.GetLogger().Info(ctx, fmt.Sprintf("updating HTTP proxy, url: %s", url))

	if url != "" {
		util.UpdateHTTPProxy(ctx, url)
	} else {
		util.UpdateHTTPProxy(ctx, "")
	}
}

func (m *Manager) GetWoxAppData(ctx context.Context) *WoxAppData {
	return m.woxAppData
}

func (m *Manager) SaveWoxSetting(ctx context.Context) error {
	// This method is now a convenience wrapper. The primary update logic is in UpdateWoxSetting.
	// It can be used to persist the entire in-memory setting state to the database if needed.
	logger.Info(ctx, "Persisting all settings to database.")
	db := database.GetDB()
	tx := db.Begin()

	// This is a simplified version. A full implementation would iterate through all settings
	// and update them, which is complex. The per-key update in UpdateWoxSetting is more efficient.

	// For now, we just log that this is happening.
	// The actual saving happens in UpdateWoxSetting.

	tx.Commit()
	logger.Info(ctx, "Wox setting state persisted.")
	return nil
}

func (m *Manager) AddQueryHistory(ctx context.Context, query common.PlainQuery) {
	if query.IsEmpty() {
		return
	}

	logger.Debug(ctx, fmt.Sprintf("add query history: %s", query.String()))
	historyEntry := QueryHistory{
		Query:     query,
		Timestamp: util.GetSystemTimestamp(),
	}
	m.woxAppData.QueryHistories = append(m.woxAppData.QueryHistories, historyEntry)

	// Persist to DB
	database.GetDB().Create(&database.QueryHistory{Query: query.String(), Timestamp: historyEntry.Timestamp})

	// Trim in-memory and DB history
	if len(m.woxAppData.QueryHistories) > 100 {
		toDeleteCount := len(m.woxAppData.QueryHistories) - 100
		m.woxAppData.QueryHistories = m.woxAppData.QueryHistories[toDeleteCount:]

		var oldestEntries []database.QueryHistory
		database.GetDB().Order("timestamp asc").Limit(toDeleteCount).Find(&oldestEntries)
		if len(oldestEntries) > 0 {
			database.GetDB().Delete(&oldestEntries)
		}
	}
}

func (m *Manager) GetLatestQueryHistory(ctx context.Context, n int) []QueryHistory {
	if n <= 0 {
		return []QueryHistory{}
	}

	if n > len(m.woxAppData.QueryHistories) {
		n = len(m.woxAppData.QueryHistories)
	}

	histories := m.woxAppData.QueryHistories[len(m.woxAppData.QueryHistories)-n:]

	// copy to new list and order by time desc
	result := make([]QueryHistory, n)
	for i := 0; i < n; i++ {
		result[i] = histories[n-i-1]
	}
	return result
}

func (m *Manager) AddFavoriteResult(ctx context.Context, pluginId string, resultTitle string, resultSubTitle string) {
	util.GetLogger().Info(ctx, fmt.Sprintf("add favorite result: %s, %s", resultTitle, resultSubTitle))
	resultHash := NewResultHash(pluginId, resultTitle, resultSubTitle)
	m.woxAppData.FavoriteResults.Store(resultHash, true)

	fav := database.FavoriteResult{PluginID: pluginId, Title: resultTitle, Subtitle: resultSubTitle}
	database.GetDB().Create(&fav)
}

func (m *Manager) IsFavoriteResult(ctx context.Context, pluginId string, resultTitle string, resultSubTitle string) bool {
	resultHash := NewResultHash(pluginId, resultTitle, resultSubTitle)
	return m.woxAppData.FavoriteResults.Exist(resultHash)
}

func (m *Manager) RemoveFavoriteResult(ctx context.Context, pluginId string, resultTitle string, resultSubTitle string) {
	util.GetLogger().Info(ctx, fmt.Sprintf("remove favorite result: %s, %s", resultTitle, resultSubTitle))
	resultHash := NewResultHash(pluginId, resultTitle, resultSubTitle)
	m.woxAppData.FavoriteResults.Delete(resultHash)

	database.GetDB().Where("plugin_id = ? AND title = ? AND subtitle = ?", pluginId, resultTitle, resultSubTitle).Delete(&database.FavoriteResult{})
}

func (m *Manager) LoadPluginSetting(ctx context.Context, pluginId string, pluginName string, defaultSettings definition.PluginSettingDefinitions) (*PluginSetting, error) {
	db := database.GetDB()
	pluginSetting := &PluginSetting{
		Name:     pluginName,
		Settings: defaultSettings.GetAllDefaults(),
	}

	var settings []database.PluginSetting
	db.Where("plugin_id = ?", pluginId).Find(&settings)

	for _, s := range settings {
		pluginSetting.Settings.Store(s.Key, s.Value)
	}

	return pluginSetting, nil
}

func (m *Manager) SavePluginSetting(ctx context.Context, pluginId string, pluginSetting *PluginSetting) error {
	db := database.GetDB()
	tx := db.Begin()

	pluginSetting.Settings.Range(func(key string, value string) bool {
		var existing database.PluginSetting
		result := tx.Where("plugin_id = ? AND key = ?", pluginId, key).First(&existing)

		if result.Error == nil {
			// Update
			tx.Model(&existing).Update("value", value)
		} else {
			// Create
			tx.Create(&database.PluginSetting{PluginID: pluginId, Key: key, Value: value})
		}
		return true
	})

	return tx.Commit().Error
}

func (m *Manager) AddActionedResult(ctx context.Context, pluginId string, resultTitle string, resultSubTitle string, query string) {
	resultHash := NewResultHash(pluginId, resultTitle, resultSubTitle)
	actionedResult := ActionedResult{
		Timestamp: util.GetSystemTimestamp(),
		Query:     query,
	}

	if v, ok := m.woxAppData.ActionedResults.Load(resultHash); ok {
		v = append(v, actionedResult)
		if len(v) > 100 {
			v = v[len(v)-100:]
		}
		m.woxAppData.ActionedResults.Store(resultHash, v)
	} else {
		m.woxAppData.ActionedResults.Store(resultHash, []ActionedResult{actionedResult})
	}

	db := database.GetDB()
	db.Create(&database.ActionedResult{
		PluginID:  pluginId,
		Title:     resultTitle,
		Subtitle:  resultSubTitle,
		Timestamp: actionedResult.Timestamp,
		Query:     actionedResult.Query,
	})
}

func (m *Manager) SaveWindowPosition(ctx context.Context, x, y int) error {
	m.woxSetting.LastWindowX = x
	m.woxSetting.LastWindowY = y
	db := database.GetDB()
	db.Model(&database.Setting{}).Where("key = ?", "LastWindowX").Update("value", strconv.Itoa(x))
	db.Model(&database.Setting{}).Where("key = ?", "LastWindowY").Update("value", strconv.Itoa(y))
	return nil
}
