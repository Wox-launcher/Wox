package setting

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
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
	woxSettingPath := util.GetLocation().GetWoxSettingPath()
	if _, statErr := os.Stat(woxSettingPath); os.IsNotExist(statErr) {
		settingJson, marshalErr := json.Marshal(WoxSetting{
			MainHotkey: m.getDefaultMainHotkey(ctx),
		})
		if marshalErr != nil {
			return marshalErr
		}

		writeErr := os.WriteFile(woxSettingPath, settingJson, 0644)
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

func (m *Manager) getDefaultMainHotkey(ctx context.Context) string {
	combineKey := "alt+space"
	if strings.ToLower(runtime.GOOS) == "darwin" {
		combineKey = "command+space"
	}
	return combineKey
}
