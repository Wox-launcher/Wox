package setting

import (
	"context"
	"fmt"
	"sync"
	"wox/common"
	"wox/database"
	"wox/util"
	"wox/util/autostart"
)

var managerInstance *Manager
var managerOnce sync.Once
var logger *util.Log

type Manager struct {
	woxSetting *WoxSetting
}

func GetSettingManager() *Manager {
	managerOnce.Do(func() {
		logger = util.GetLogger()
		db := database.GetDB()
		if db == nil {
			logger.Error(context.Background(), "Database not initialized, cannot create Setting Manager")
			panic("database not initialized")
		}

		store := NewWoxSettingStore(db)
		managerInstance = &Manager{}
		managerInstance.woxSetting = NewWoxSetting(store)
	})
	return managerInstance
}

func (m *Manager) Init(ctx context.Context) error {
	m.StartAutoBackup(ctx)

	if err := m.checkAutostart(ctx); err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to check autostart status: %v", err))
	}

	return nil
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
	}
	return nil
}

func (m *Manager) GetWoxSetting(ctx context.Context) *WoxSetting {
	return m.woxSetting
}

func (m *Manager) GetLatestQueryHistory(ctx context.Context, limit int) []common.PlainQuery {
	histories := m.woxSetting.QueryHistories.Get()

	// Sort by timestamp descending and limit results
	var result []common.PlainQuery
	count := 0
	for i := len(histories) - 1; i >= 0 && count < limit; i-- {
		result = append(result, histories[i].Query)
		count++
	}

	return result
}

func (m *Manager) LoadPluginSetting(ctx context.Context, pluginId string, defaultSettings map[string]string) (*PluginSetting, error) {
	pluginSettingStore := NewPluginSettingStore(database.GetDB(), pluginId)
	pluginSetting := NewPluginSetting(pluginSettingStore, defaultSettings)
	return pluginSetting, nil
}

func (m *Manager) AddActionedResult(ctx context.Context, pluginId string, resultTitle string, resultSubTitle string, query string) {
	resultHash := NewResultHash(pluginId, resultTitle, resultSubTitle)
	actionedResult := ActionedResult{
		Timestamp: util.GetSystemTimestamp(),
		Query:     query,
	}

	actionedResults := m.woxSetting.ActionedResults.Get()
	if v, ok := actionedResults.Load(resultHash); ok {
		v = append(v, actionedResult)
		if len(v) > 100 {
			v = v[len(v)-100:]
		}
		actionedResults.Store(resultHash, v)
	} else {
		actionedResults.Store(resultHash, []ActionedResult{actionedResult})
	}
	m.woxSetting.ActionedResults.Set(actionedResults)
}

func (m *Manager) AddFavoriteResult(ctx context.Context, pluginId string, resultTitle string, resultSubTitle string) {
	util.GetLogger().Info(ctx, fmt.Sprintf("add favorite result: %s, %s", resultTitle, resultSubTitle))
	resultHash := NewResultHash(pluginId, resultTitle, resultSubTitle)
	favoriteResults := m.woxSetting.FavoriteResults.Get()
	favoriteResults.Store(resultHash, true)
	m.woxSetting.FavoriteResults.Set(favoriteResults)
}

func (m *Manager) IsFavoriteResult(ctx context.Context, pluginId string, resultTitle string, resultSubTitle string) bool {
	resultHash := NewResultHash(pluginId, resultTitle, resultSubTitle)
	return m.woxSetting.FavoriteResults.Get().Exist(resultHash)
}

func (m *Manager) RemoveFavoriteResult(ctx context.Context, pluginId string, resultTitle string, resultSubTitle string) {
	util.GetLogger().Info(ctx, fmt.Sprintf("remove favorite result: %s, %s", resultTitle, resultSubTitle))
	resultHash := NewResultHash(pluginId, resultTitle, resultSubTitle)
	favoriteResults := m.woxSetting.FavoriteResults.Get()
	favoriteResults.Delete(resultHash)
	m.woxSetting.FavoriteResults.Set(favoriteResults)
}
