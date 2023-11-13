package setting

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"wox/util"
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
	woxSettingErr := m.loadWoxSetting(ctx)
	if woxSettingErr != nil {
		return woxSettingErr
	}

	woxAppDataErr := m.loadWoxAppData(ctx)
	if woxAppDataErr != nil {
		// wox app data is not essential, so we just log the error
		logger.Error(ctx, fmt.Sprintf("failed to load wox app data: %s", woxAppDataErr.Error()))
	}

	return nil
}

func (m *Manager) loadWoxSetting(ctx context.Context) error {
	defaultWoxSetting := GetDefaultWoxSetting(ctx)

	woxSettingPath := util.GetLocation().GetWoxSettingPath()
	if _, statErr := os.Stat(woxSettingPath); os.IsNotExist(statErr) {
		defaultWoxSettingJson, marshalErr := json.Marshal(defaultWoxSetting)
		if marshalErr != nil {
			return marshalErr
		}

		writeErr := os.WriteFile(woxSettingPath, defaultWoxSettingJson, 0644)
		if writeErr != nil {
			return writeErr
		}
	}

	woxSettingFile, openErr := os.Open(woxSettingPath)
	if openErr != nil {
		return openErr
	}
	defer woxSettingFile.Close()

	woxSetting := &WoxSetting{}
	decodeErr := json.NewDecoder(woxSettingFile).Decode(woxSetting)
	if decodeErr != nil {
		return decodeErr
	}
	// some settings were added later, json file may not have them, so we need to set them to default value
	if woxSetting.MainHotkey.Get() == "" {
		woxSetting.MainHotkey.Set(defaultWoxSetting.MainHotkey.Get())
	}
	if woxSetting.LangCode == "" {
		woxSetting.LangCode = defaultWoxSetting.LangCode
	}
	if woxSetting.LastQueryMode == "" {
		woxSetting.LastQueryMode = defaultWoxSetting.LastQueryMode
	}
	if woxSetting.ThemeId == "" {
		woxSetting.ThemeId = defaultWoxSetting.ThemeId
	}

	m.woxSetting = woxSetting

	return nil
}

func (m *Manager) loadWoxAppData(ctx context.Context) error {
	woxAppDataPath := util.GetLocation().GetWoxAppDataPath()
	if _, statErr := os.Stat(woxAppDataPath); os.IsNotExist(statErr) {
		defaultWoxAppData := GetDefaultWoxAppData(ctx)
		defaultWoxAppDataJson, marshalErr := json.Marshal(defaultWoxAppData)
		if marshalErr != nil {
			return marshalErr
		}

		writeErr := os.WriteFile(woxAppDataPath, defaultWoxAppDataJson, 0644)
		if writeErr != nil {
			return writeErr
		}
	}

	woxAppDataFile, openErr := os.Open(woxAppDataPath)
	if openErr != nil {
		return openErr
	}
	defer woxAppDataFile.Close()

	woxAppData := &WoxAppData{}
	decodeErr := json.NewDecoder(woxAppDataFile).Decode(woxAppData)
	if decodeErr != nil {
		return decodeErr
	}

	m.woxAppData = woxAppData

	return nil
}

func (m *Manager) GetWoxSetting(ctx context.Context) *WoxSetting {
	return m.woxSetting
}

func (m *Manager) UpdateWoxSetting(ctx context.Context, setting WoxSetting) error {
	m.woxSetting = &setting
	return m.SaveWoxSetting(ctx)
}

func (m *Manager) GetWoxAppData(ctx context.Context) *WoxAppData {
	return m.woxAppData
}

func (m *Manager) SaveWoxSetting(ctx context.Context) error {
	woxSettingPath := util.GetLocation().GetWoxSettingPath()
	settingJson, marshalErr := json.Marshal(m.woxSetting)
	if marshalErr != nil {
		logger.Error(ctx, marshalErr.Error())
		return marshalErr
	}

	writeErr := os.WriteFile(woxSettingPath, settingJson, 0644)
	if writeErr != nil {
		logger.Error(ctx, writeErr.Error())
		return writeErr
	}

	logger.Info(ctx, "Wox setting saved")
	return nil
}

func (m *Manager) saveWoxAppData(ctx context.Context) error {
	woxAppDataPath := util.GetLocation().GetWoxAppDataPath()
	settingJson, marshalErr := json.Marshal(m.woxAppData)
	if marshalErr != nil {
		logger.Error(ctx, marshalErr.Error())
		return marshalErr
	}

	writeErr := os.WriteFile(woxAppDataPath, settingJson, 0644)
	if writeErr != nil {
		logger.Error(ctx, writeErr.Error())
		return writeErr
	}

	logger.Info(ctx, "Wox setting saved")
	return nil
}

func (m *Manager) LoadPluginSetting(ctx context.Context, pluginId string, defaultSettings CustomizedPluginSettings) (*PluginSetting, error) {
	pluginSettingPath := path.Join(util.GetLocation().GetPluginSettingDirectory(), fmt.Sprintf("%s.json", pluginId))
	if _, statErr := os.Stat(pluginSettingPath); os.IsNotExist(statErr) {
		return &PluginSetting{
			CustomizedSettings: defaultSettings.GetAll(),
		}, nil
	}

	fileContent, readErr := os.ReadFile(pluginSettingPath)
	if readErr != nil {
		return &PluginSetting{}, readErr
	}

	var pluginSetting = &PluginSetting{}
	decodeErr := json.Unmarshal(fileContent, pluginSetting)
	if decodeErr != nil {
		return &PluginSetting{}, decodeErr
	}

	return pluginSetting, nil
}

func (m *Manager) SavePluginSetting(ctx context.Context, pluginId string, pluginSetting *PluginSetting) error {
	pluginSettingPath := path.Join(util.GetLocation().GetPluginSettingDirectory(), fmt.Sprintf("%s.json", pluginId))
	pluginSettingJson, marshalErr := json.Marshal(pluginSetting)
	if marshalErr != nil {
		logger.Error(ctx, marshalErr.Error())
		return marshalErr
	}

	writeErr := os.WriteFile(pluginSettingPath, pluginSettingJson, 0644)
	if writeErr != nil {
		logger.Error(ctx, writeErr.Error())
		return writeErr
	}

	logger.Info(ctx, fmt.Sprintf("plugin setting saved: %s", pluginId))
	return nil
}

func (m *Manager) AddQueryHistory(ctx context.Context, query string) {
	if strings.TrimSpace(query) == "" {
		return
	}

	logger.Debug(ctx, fmt.Sprintf("add query history: %s", query))
	m.woxAppData.QueryHistories = append(m.woxAppData.QueryHistories, QueryHistory{
		Query:     query,
		Timestamp: util.GetSystemTimestamp(),
	})

	// if query history is more than 100, remove the oldest ones
	if len(m.woxAppData.QueryHistories) > 100 {
		m.woxAppData.QueryHistories = m.woxAppData.QueryHistories[len(m.woxAppData.QueryHistories)-100:]
	}

	m.saveWoxAppData(ctx)
}

func (m *Manager) AddActionedResult(ctx context.Context, pluginId string, resultTitle string, resultSubTitle string) {
	resultHash := NewResultHash(pluginId, resultTitle, resultSubTitle)
	actionedResult := ActionedResult{Timestamp: util.GetSystemTimestamp()}

	if v, ok := m.woxAppData.ActionedResults.Load(resultHash); ok {
		v = append(v, actionedResult)
		// if current hash actioned results is more than 100, remove the oldest ones
		if len(v) > 100 {
			v = v[len(v)-100:]
		}
		m.woxAppData.ActionedResults.Store(resultHash, v)
	} else {
		m.woxAppData.ActionedResults.Store(resultHash, []ActionedResult{actionedResult})
	}

	m.saveWoxAppData(ctx)
}
