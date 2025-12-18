package migration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"
	"wox/common"
	"wox/i18n"
	"wox/setting"
	"wox/util"
	"wox/util/locale"

	_ "github.com/mattn/go-sqlite3"
	"gorm.io/gorm"
)

func init() {
	Register(&legacyJsonToSqliteMigration{})
}

type legacyJsonToSqliteMigration struct {
	filesToBackup               []string
	shouldDeleteClipboardDbFavs bool
	clipboardPluginId           string
}

func (m *legacyJsonToSqliteMigration) ID() string { return "20251218_legacy_json_to_sqlite" }

func (m *legacyJsonToSqliteMigration) Description() string {
	return "Migrate legacy JSON settings/app data (and clipboard favorites) into SQLite-backed setting stores"
}

func (m *legacyJsonToSqliteMigration) IsNeeded(ctx context.Context, db *gorm.DB) (bool, error) {
	oldSettingPath := util.GetLocation().GetWoxSettingPath()
	oldAppDataPath := util.GetLocation().GetWoxAppDataPath()

	if _, err := os.Stat(oldSettingPath); err == nil {
		return true, nil
	}
	if _, err := os.Stat(oldAppDataPath); err == nil {
		return true, nil
	}

	pluginDir := util.GetLocation().GetPluginSettingDirectory()
	entries, err := os.ReadDir(pluginDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			if strings.Contains(e.Name(), "wox") {
				continue
			}
			return true, nil
		}
	}

	clipboardPluginId := "5f815d98-27f5-488d-a756-c317ea39935b"
	clipboardDbPath := path.Join(pluginDir, clipboardPluginId+"_clipboard.db")
	if _, err := os.Stat(clipboardDbPath); err == nil {
		return true, nil
	}

	return false, nil
}

func (m *legacyJsonToSqliteMigration) Up(ctx context.Context, tx *gorm.DB) error {
	logger := util.GetLogger()

	oldSettingPath := util.GetLocation().GetWoxSettingPath()
	oldAppDataPath := util.GetLocation().GetWoxAppDataPath()

	oldSettings := getOldDefaultWoxSetting()
	if content, err := os.ReadFile(oldSettingPath); err == nil && len(content) > 0 {
		if unmarshalErr := json.Unmarshal(content, &oldSettings); unmarshalErr != nil {
			logger.Warn(ctx, fmt.Sprintf("failed to unmarshal old wox.setting.json: %v, will use defaults for migration.", unmarshalErr))
		}
	}

	var oldAppData oldWoxAppData
	oldAppData.QueryHistories = []oldQueryHistory{}
	if content, err := os.ReadFile(oldAppDataPath); err == nil && len(content) > 0 {
		if json.Unmarshal(content, &oldAppData) != nil {
			logger.Warn(ctx, "failed to unmarshal old wox.app.data.json, will use defaults for migration.")
		}
	}

	woxSettingStore := setting.NewWoxSettingStore(tx)

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
		"QueryMode":            oldSettings.LastQueryMode,
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
		"QueryHotkeys":         oldSettings.QueryHotkeys,
		"QueryShortcuts":       oldSettings.QueryShortcuts,
		"AIProviders":          oldSettings.AIProviders,
	}

	for key, value := range settingsToMigrate {
		if err := woxSettingStore.Set(key, value); err != nil {
			return fmt.Errorf("failed to migrate setting %s: %w", key, err)
		}
	}

	pluginDir := util.GetLocation().GetPluginSettingDirectory()
	entries, err := os.ReadDir(pluginDir)
	if err == nil {
		for _, file := range entries {
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
			pluginSettingStore := setting.NewPluginSettingStore(tx, pluginId)

			pluginJsonPath := path.Join(pluginDir, file.Name())
			content, err := os.ReadFile(pluginJsonPath)
			if err != nil {
				continue
			}
			var legacySetting struct {
				Name     string            `json:"Name"`
				Settings map[string]string `json:"Settings"`
			}
			if err := json.Unmarshal(content, &legacySetting); err != nil {
				continue
			}

			for key, value := range legacySetting.Settings {
				if value == "" {
					continue
				}
				if err := pluginSettingStore.Set(key, value); err != nil {
					logger.Warn(ctx, fmt.Sprintf("failed to migrate plugin setting %s for %s: %v", key, pluginId, err))
					continue
				}
			}

			m.filesToBackup = append(m.filesToBackup, pluginJsonPath)
		}
	}

	if len(oldAppData.QueryHistories) > 0 {
		if err := woxSettingStore.Set("QueryHistories", oldAppData.QueryHistories); err != nil {
			logger.Warn(ctx, fmt.Sprintf("failed to migrate query histories: %v", err))
		}
	}
	if oldAppData.FavoriteResults != nil {
		if err := woxSettingStore.Set("FavoriteResults", oldAppData.FavoriteResults); err != nil {
			logger.Warn(ctx, fmt.Sprintf("failed to migrate favorite results: %v", err))
		}
	}

	if err := m.migrateClipboardData(ctx, tx); err != nil {
		logger.Warn(ctx, fmt.Sprintf("failed to migrate clipboard data: %v", err))
	}

	if _, err := os.Stat(oldSettingPath); err == nil {
		m.filesToBackup = append(m.filesToBackup, oldSettingPath)
	}
	if _, err := os.Stat(oldAppDataPath); err == nil {
		m.filesToBackup = append(m.filesToBackup, oldAppDataPath)
	}

	return nil
}

func (m *legacyJsonToSqliteMigration) AfterCommit(ctx context.Context) error {
	logger := util.GetLogger()

	for _, p := range m.filesToBackup {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if err := os.Rename(p, p+".bak"); err != nil {
			logger.Warn(ctx, fmt.Sprintf("failed to rename legacy file to .bak: %s: %v", p, err))
		}
	}

	if m.shouldDeleteClipboardDbFavs && m.clipboardPluginId != "" {
		if _, err := deleteFavoritesFromDatabase(ctx, m.clipboardPluginId); err != nil {
			logger.Warn(ctx, fmt.Sprintf("failed to delete clipboard favorites from database: %v", err))
		}
	}

	return nil
}

type oldPlatformSettingValue[T any] struct {
	WinValue   T `json:"WinValue"`
	MacValue   T `json:"MacValue"`
	LinuxValue T `json:"LinuxValue"`
}

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

type oldQueryHotkey struct {
	Hotkey            string
	Query             string
	IsSilentExecution bool
}

type oldQueryShortcut struct {
	Shortcut string
	Query    string
}

type oldAIProvider struct {
	Name   common.ProviderName
	ApiKey string
	Host   string
}

type oldQueryHistory struct {
	Query     common.PlainQuery
	Timestamp int64
}

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

type oldClipboardHistory struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Type       string `json:"type"`
	Timestamp  int64  `json:"timestamp"`
	ImagePath  string `json:"imagePath,omitempty"`
	IsFavorite bool   `json:"isFavorite,omitempty"`
}

type favoriteClipboardItem struct {
	ID        string  `json:"id"`
	Type      string  `json:"type"`
	Content   string  `json:"content"`
	FilePath  string  `json:"filePath,omitempty"`
	IconData  *string `json:"iconData,omitempty"`
	Width     *int    `json:"width,omitempty"`
	Height    *int    `json:"height,omitempty"`
	FileSize  *int64  `json:"fileSize,omitempty"`
	Timestamp int64   `json:"timestamp"`
	CreatedAt int64   `json:"createdAt"`
}

type clipboardRecord struct {
	ID         string
	Type       string
	Content    string
	FilePath   string
	IconData   *string
	Width      *int
	Height     *int
	FileSize   *int64
	Timestamp  int64
	IsFavorite bool
	CreatedAt  time.Time
}

func (m *legacyJsonToSqliteMigration) migrateClipboardData(ctx context.Context, tx *gorm.DB) error {
	clipboardPluginId := "5f815d98-27f5-488d-a756-c317ea39935b"
	m.clipboardPluginId = clipboardPluginId

	pluginSettingStore := setting.NewPluginSettingStore(tx, clipboardPluginId)

	var allFavoritesToMigrate []favoriteClipboardItem

	var historyJson string
	err := pluginSettingStore.Get("history", &historyJson)
	if err == nil && historyJson != "" {
		var history []oldClipboardHistory
		if json.Unmarshal([]byte(historyJson), &history) == nil {
			for _, item := range history {
				if item.IsFavorite {
					allFavoritesToMigrate = append(allFavoritesToMigrate, favoriteClipboardItem{
						ID:        item.ID,
						Type:      item.Type,
						Content:   item.Text,
						FilePath:  item.ImagePath,
						Timestamp: item.Timestamp,
						CreatedAt: item.Timestamp / 1000,
					})
				}
			}
		}
		_ = pluginSettingStore.Set("history", "")
	}

	dbFavorites, err := getFavoritesFromDatabase(ctx, clipboardPluginId)
	if err == nil && len(dbFavorites) > 0 {
		for _, record := range dbFavorites {
			allFavoritesToMigrate = append(allFavoritesToMigrate, favoriteClipboardItem{
				ID:        record.ID,
				Type:      record.Type,
				Content:   record.Content,
				FilePath:  record.FilePath,
				IconData:  record.IconData,
				Width:     record.Width,
				Height:    record.Height,
				FileSize:  record.FileSize,
				Timestamp: record.Timestamp,
				CreatedAt: record.CreatedAt.Unix(),
			})
		}
		m.shouldDeleteClipboardDbFavs = true
	}

	if len(allFavoritesToMigrate) == 0 {
		return nil
	}

	favoritesJson, err := json.Marshal(allFavoritesToMigrate)
	if err != nil {
		return fmt.Errorf("failed to marshal favorites: %w", err)
	}
	if err := pluginSettingStore.Set("favorites", string(favoritesJson)); err != nil {
		return fmt.Errorf("failed to save migrated favorites: %w", err)
	}
	return nil
}

func getFavoritesFromDatabase(ctx context.Context, pluginId string) ([]clipboardRecord, error) {
	dbPath := path.Join(util.GetLocation().GetPluginSettingDirectory(), pluginId+"_clipboard.db")

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return []clipboardRecord{}, nil
	}

	dsn := dbPath + "?" +
		"_journal_mode=WAL&" +
		"_synchronous=NORMAL&" +
		"_cache_size=1000&" +
		"_foreign_keys=true&" +
		"_busy_timeout=5000"

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open clipboard database: %w", err)
	}
	defer db.Close()

	querySQL := `
	SELECT id, type, content, file_path, icon_data, width, height, file_size, timestamp, is_favorite, created_at
	FROM clipboard_history
	WHERE is_favorite = TRUE
	ORDER BY timestamp DESC
	`

	rows, err := db.QueryContext(ctx, querySQL)
	if err != nil {
		return nil, fmt.Errorf("failed to query favorites: %w", err)
	}
	defer rows.Close()

	var records []clipboardRecord
	for rows.Next() {
		var record clipboardRecord
		if err := rows.Scan(&record.ID, &record.Type, &record.Content,
			&record.FilePath, &record.IconData, &record.Width, &record.Height, &record.FileSize,
			&record.Timestamp, &record.IsFavorite, &record.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan record: %w", err)
		}
		records = append(records, record)
	}

	return records, rows.Err()
}

func deleteFavoritesFromDatabase(ctx context.Context, pluginId string) (int64, error) {
	dbPath := path.Join(util.GetLocation().GetPluginSettingDirectory(), pluginId+"_clipboard.db")

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return 0, nil
	}

	dsn := dbPath + "?" +
		"_journal_mode=WAL&" +
		"_synchronous=NORMAL&" +
		"_cache_size=1000&" +
		"_foreign_keys=true&" +
		"_busy_timeout=5000"

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return 0, fmt.Errorf("failed to open clipboard database: %w", err)
	}
	defer db.Close()

	deleteSQL := `DELETE FROM clipboard_history WHERE is_favorite = TRUE`
	result, err := db.ExecContext(ctx, deleteSQL)
	if err != nil {
		return 0, fmt.Errorf("failed to delete favorites: %w", err)
	}

	return result.RowsAffected()
}
