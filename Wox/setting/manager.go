package setting

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"sync"
	"wox/util"
)

var managerInstance *Manager
var managerOnce sync.Once
var logger *util.Log

type Manager struct {
	woxSetting WoxSetting
}

func GetSettingManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{}
		logger = util.GetLogger()
	})
	return managerInstance
}

func (m *Manager) Init(ctx context.Context) error {
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

	woxSetting := WoxSetting{}
	decodeErr := json.NewDecoder(woxSettingFile).Decode(&woxSetting)
	if decodeErr != nil {
		return decodeErr
	}
	if woxSetting.LangCode == "" {
		woxSetting.LangCode = defaultWoxSetting.LangCode
	}

	//sort query history ascending
	sort.Slice(woxSetting.QueryHistories, func(i, j int) bool {
		return woxSetting.QueryHistories[i].Timestamp < woxSetting.QueryHistories[j].Timestamp
	})

	m.woxSetting = woxSetting

	return nil
}

func (m *Manager) GetWoxSetting(ctx context.Context) WoxSetting {
	return m.woxSetting
}

func (m *Manager) SaveWoxSetting(ctx context.Context, woxSetting WoxSetting) error {
	woxSettingPath := util.GetLocation().GetWoxSettingPath()
	settingJson, marshalErr := json.Marshal(woxSetting)
	if marshalErr != nil {
		logger.Error(ctx, marshalErr.Error())
		return marshalErr
	}

	writeErr := os.WriteFile(woxSettingPath, settingJson, 0644)
	if writeErr != nil {
		logger.Error(ctx, writeErr.Error())
		return writeErr
	}

	m.woxSetting = woxSetting

	logger.Info(ctx, "Wox setting saved")
	return nil
}

func (m *Manager) LoadPluginSetting(ctx context.Context, pluginId string, defaultSettings CustomizedPluginSettings) (pluginSetting PluginSetting, err error) {
	pluginSettingPath := path.Join(util.GetLocation().GetPluginSettingDirectory(), fmt.Sprintf("%s.json", pluginId))
	if _, statErr := os.Stat(pluginSettingPath); os.IsNotExist(statErr) {
		pluginSetting = PluginSetting{
			CustomizedSettings: defaultSettings.GetAll(),
		}
		return pluginSetting, nil
	}

	fileContent, readErr := os.ReadFile(pluginSettingPath)
	if readErr != nil {
		return PluginSetting{}, readErr
	}

	decodeErr := json.Unmarshal(fileContent, &pluginSetting)
	if decodeErr != nil {
		return PluginSetting{}, decodeErr
	}

	return pluginSetting, nil
}

func (m *Manager) SavePluginSetting(ctx context.Context, pluginId string, pluginSetting PluginSetting) error {
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
	logger.Debug(ctx, fmt.Sprintf("add query history: %s", query))
	woxSetting := m.GetWoxSetting(ctx)
	woxSetting.QueryHistories = append(woxSetting.QueryHistories, QueryHistory{
		Query:     query,
		Timestamp: util.GetSystemTimestamp(),
	})

	// if query history is more than 100, remove the oldest ones
	if len(woxSetting.QueryHistories) > 100 {
		woxSetting.QueryHistories = woxSetting.QueryHistories[len(woxSetting.QueryHistories)-100:]
	}

	m.SaveWoxSetting(ctx, woxSetting)
}
