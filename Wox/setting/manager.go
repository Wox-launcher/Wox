package setting

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"slices"
	"sync"
	"wox/i18n"
	"wox/setting/definition"
	"wox/share"
	"wox/util"
	"wox/util/autostart"
	"wox/util/hotkey"

	"github.com/tidwall/pretty"
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
		// wox app data is not essential, so we just log the error and use default value
		logger.Error(ctx, fmt.Sprintf("failed to load wox app data: %s", woxAppDataErr.Error()))
		defaultWoxAppData := GetDefaultWoxAppData(ctx)
		m.woxAppData = &defaultWoxAppData
	}

	m.StartAutoBackup(ctx)

	//check autostart status, if not match, update the setting
	actualAutostart, err := autostart.IsAutostart(ctx)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("Failed to check autostart status: %s", err.Error()))
	} else {
		configAutostart := m.woxSetting.EnableAutostart.Get()
		if actualAutostart != configAutostart {
			util.GetLogger().Warn(ctx, fmt.Sprintf("Autostart setting mismatch: config %v, actual %v. Updating config.", configAutostart, actualAutostart))
			m.woxSetting.EnableAutostart.Set(actualAutostart)
			err := m.SaveWoxSetting(ctx)
			if err != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("Failed to save updated autostart setting: %s", err.Error()))
			}
		}
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

		writeErr := os.WriteFile(woxSettingPath, pretty.Pretty(defaultWoxSettingJson), 0644)
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
	if woxSetting.SelectionHotkey.Get() == "" {
		woxSetting.SelectionHotkey.Set(defaultWoxSetting.SelectionHotkey.Get())
	}
	if woxSetting.LangCode == "" {
		woxSetting.LangCode = defaultWoxSetting.LangCode
	}
	if woxSetting.LastQueryMode == "" {
		woxSetting.LastQueryMode = defaultWoxSetting.LastQueryMode
	}
	if woxSetting.AppWidth == 0 {
		woxSetting.AppWidth = defaultWoxSetting.AppWidth
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

		writeErr := os.WriteFile(woxAppDataPath, pretty.Pretty(defaultWoxAppDataJson), 0644)
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
	if woxAppData.ActionedResults == nil {
		woxAppData.ActionedResults = util.NewHashMap[ResultHash, []ActionedResult]()
	}

	// sort query histories by timestamp asc
	slices.SortFunc(woxAppData.QueryHistories, func(i, j QueryHistory) int {
		return int(i.Timestamp - j.Timestamp)
	})

	m.woxAppData = woxAppData

	return nil
}

func (m *Manager) GetWoxSetting(ctx context.Context) *WoxSetting {
	return m.woxSetting
}

func (m *Manager) UpdateWoxSetting(ctx context.Context, key, value string) error {
	if key == "" {
		return fmt.Errorf("key is empty")
	}

	if key == "EnableAutostart" {
		m.woxSetting.EnableAutostart.Set(value == "true")
	} else if key == "MainHotkey" {
		if value != "" {
			isAvailable := hotkey.IsHotkeyAvailable(ctx, value)
			if !isAvailable {
				return fmt.Errorf("hotkey is not available: %s", value)
			}
		}
		m.woxSetting.MainHotkey.Set(value)
	} else if key == "SelectionHotkey" {
		isAvailable := hotkey.IsHotkeyAvailable(ctx, value)
		if !isAvailable {
			return fmt.Errorf("hotkey is not available: %s", value)
		}
		m.woxSetting.SelectionHotkey.Set(value)
	} else if key == "UsePinYin" {
		m.woxSetting.UsePinYin = value == "true"
	} else if key == "SwitchInputMethodABC" {
		m.woxSetting.SwitchInputMethodABC = value == "true"
	} else if key == "HideOnStart" {
		m.woxSetting.HideOnStart = value == "true"
	} else if key == "HideOnLostFocus" {
		m.woxSetting.HideOnLostFocus = value == "true"
	} else if key == "ShowTray" {
		m.woxSetting.ShowTray = value == "true"
	} else if key == "LangCode" {
		newLangCode := i18n.LangCode(value)
		langErr := i18n.GetI18nManager().UpdateLang(ctx, newLangCode)
		if langErr != nil {
			return langErr
		}
		m.woxSetting.LangCode = newLangCode
	} else if key == "LastQueryMode" {
		m.woxSetting.LastQueryMode = value
	} else if key == "ThemeId" {
		m.woxSetting.ThemeId = value
	} else if key == "QueryHotkeys" {
		// value is a json string
		var queryHotkeys []QueryHotkey
		if unmarshalErr := json.Unmarshal([]byte(value), &queryHotkeys); unmarshalErr != nil {
			return unmarshalErr
		}
		m.woxSetting.QueryHotkeys.Set(queryHotkeys)
	} else if key == "QueryShortcuts" {
		// value is a json string
		var queryShortcuts []QueryShortcut
		if unmarshalErr := json.Unmarshal([]byte(value), &queryShortcuts); unmarshalErr != nil {
			return unmarshalErr
		}

		m.woxSetting.QueryShortcuts = queryShortcuts
	} else if key == "AIProviders" {
		// value is a json string
		var aiModels []AIProvider
		if unmarshalErr := json.Unmarshal([]byte(value), &aiModels); unmarshalErr != nil {
			return unmarshalErr
		}

		m.woxSetting.AIProviders = aiModels
	} else {
		return fmt.Errorf("unknown key: %s", key)
	}

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

	writeErr := os.WriteFile(woxSettingPath, pretty.Pretty(settingJson), 0644)
	if writeErr != nil {
		logger.Error(ctx, writeErr.Error())
		return writeErr
	}

	logger.Info(ctx, "Wox setting saved")
	return nil
}

func (m *Manager) saveWoxAppData(ctx context.Context, reason string) error {
	woxAppDataPath := util.GetLocation().GetWoxAppDataPath()
	settingJson, marshalErr := json.Marshal(m.woxAppData)
	if marshalErr != nil {
		logger.Error(ctx, marshalErr.Error())
		return marshalErr
	}

	writeErr := os.WriteFile(woxAppDataPath, pretty.Pretty(settingJson), 0644)
	if writeErr != nil {
		logger.Error(ctx, writeErr.Error())
		return writeErr
	}

	logger.Info(ctx, fmt.Sprintf("Wox setting saved, reason: %s", reason))
	return nil
}

func (m *Manager) LoadPluginSetting(ctx context.Context, pluginId string, pluginName string, defaultSettings definition.PluginSettingDefinitions) (*PluginSetting, error) {
	pluginSettingPath := path.Join(util.GetLocation().GetPluginSettingDirectory(), fmt.Sprintf("%s.json", pluginId))
	if _, statErr := os.Stat(pluginSettingPath); os.IsNotExist(statErr) {
		return &PluginSetting{
			Name:     pluginName,
			Settings: defaultSettings.GetAllDefaults(),
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
	if pluginSetting.Settings == nil {
		pluginSetting.Settings = defaultSettings.GetAllDefaults()
	}

	//check if all default settings are present in the plugin settings
	//plugin author may add new definitions which are not in the user settings
	defaultSettings.GetAllDefaults().Range(func(key string, value string) bool {
		if _, exist := pluginSetting.Settings.Load(key); !exist {
			pluginSetting.Settings.Store(key, value)
		}
		return true
	})

	pluginSetting.Name = pluginName
	return pluginSetting, nil
}

func (m *Manager) SavePluginSetting(ctx context.Context, pluginId string, pluginSetting *PluginSetting) error {
	pluginSettingPath := path.Join(util.GetLocation().GetPluginSettingDirectory(), fmt.Sprintf("%s.json", pluginId))
	pluginSettingJson, marshalErr := json.Marshal(pluginSetting)
	if marshalErr != nil {
		logger.Error(ctx, marshalErr.Error())
		return marshalErr
	}

	writeErr := os.WriteFile(pluginSettingPath, pretty.Pretty(pluginSettingJson), 0644)
	if writeErr != nil {
		logger.Error(ctx, writeErr.Error())
		return writeErr
	}

	logger.Info(ctx, fmt.Sprintf("plugin setting saved: %s", pluginId))
	return nil
}

func (m *Manager) AddQueryHistory(ctx context.Context, query share.PlainQuery) {
	if query.IsEmpty() {
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

	m.saveWoxAppData(ctx, "add query history")
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

	m.saveWoxAppData(ctx, "add actioned result")
}
