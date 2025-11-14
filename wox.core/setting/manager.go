package setting

import (
	"context"
	"fmt"
	"sync"
	"time"
	"wox/common"
	"wox/database"
	"wox/util"
	"wox/util/autostart"

	"github.com/samber/lo"
)

var managerInstance *Manager
var managerOnce sync.Once
var logger *util.Log

type Manager struct {
	woxSetting *WoxSetting
	mruManager *MRUManager
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
		managerInstance.mruManager = NewMRUManager(db)
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

func (m *Manager) GetLatestQueryHistory(ctx context.Context, limit int) []QueryHistory {
	histories := m.woxSetting.QueryHistories.Get()

	// Sort by timestamp descending and limit results
	var result []QueryHistory
	count := 0
	for i := len(histories) - 1; i >= 0 && count < limit; i-- {
		result = append(result, histories[i])
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

func (m *Manager) PinResult(ctx context.Context, pluginId string, resultTitle string, resultSubTitle string) {
	util.GetLogger().Info(ctx, fmt.Sprintf("pin result: %s, %s", resultTitle, resultSubTitle))
	resultHash := NewResultHash(pluginId, resultTitle, resultSubTitle)
	results := m.woxSetting.PinedResults.Get()
	results.Store(resultHash, true)
	m.woxSetting.PinedResults.Set(results)
}

func (m *Manager) IsPinedResult(ctx context.Context, pluginId string, resultTitle string, resultSubTitle string) bool {
	resultHash := NewResultHash(pluginId, resultTitle, resultSubTitle)
	return m.woxSetting.PinedResults.Get().Exist(resultHash)
}

func (m *Manager) UnpinResult(ctx context.Context, pluginId string, resultTitle string, resultSubTitle string) {
	util.GetLogger().Info(ctx, fmt.Sprintf("unpin result: %s, %s", resultTitle, resultSubTitle))
	resultHash := NewResultHash(pluginId, resultTitle, resultSubTitle)
	results := m.woxSetting.PinedResults.Get()
	results.Delete(resultHash)
	m.woxSetting.PinedResults.Set(results)
}

func (m *Manager) AddQueryHistory(ctx context.Context, query common.PlainQuery) {
	histories := m.woxSetting.QueryHistories.Get()
	newHistory := QueryHistory{
		Query:     query,
		Timestamp: util.GetSystemTimestamp(),
	}

	// Remove duplicate if exists (same query text)
	histories = lo.Filter(histories, func(item QueryHistory, index int) bool {
		return !item.Query.IsEmpty() && item.Query.QueryText != query.QueryText
	})

	// Add new history at the end
	histories = append(histories, newHistory)

	// Keep only the most recent 1000 entries
	if len(histories) > 1000 {
		histories = histories[len(histories)-1000:]
	}

	m.woxSetting.QueryHistories.Set(histories)
}

// MRU related methods

func (m *Manager) AddMRUItem(ctx context.Context, item MRUItem) error {
	return m.mruManager.AddMRUItem(ctx, item)
}

func (m *Manager) GetMRUItems(ctx context.Context, limit int) ([]MRUItem, error) {
	return m.mruManager.GetMRUItems(ctx, limit)
}

func (m *Manager) RemoveMRUItem(ctx context.Context, pluginID, title, subTitle string) error {
	return m.mruManager.RemoveMRUItem(ctx, pluginID, title, subTitle)
}

func (m *Manager) CleanupOldMRUItems(ctx context.Context, keepCount int) error {
	return m.mruManager.CleanupOldMRUItems(ctx, keepCount)
}

// StartMRUCleanup starts a background goroutine to periodically clean up old MRU items
func (m *Manager) StartMRUCleanup(ctx context.Context) {
	util.Go(ctx, "MRU cleanup", func() {
		ticker := time.NewTicker(24 * time.Hour) // Clean up once per day
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Keep only the most recent 100 MRU items
				if err := m.CleanupOldMRUItems(ctx, 100); err != nil {
					util.GetLogger().Error(ctx, fmt.Sprintf("failed to cleanup old MRU items: %s", err.Error()))
				}
			case <-ctx.Done():
				return
			}
		}
	})
}
