package i18n

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"wox/resource"
	"wox/util"

	"github.com/tidwall/gjson"
)

var managerInstance *Manager
var managerOnce sync.Once

type Manager struct {
	currentLangCode   LangCode
	enUsLangJson      string
	currentLangJson   string
	pluginLangJsonMap util.HashMap[string, string]
}

func GetI18nManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{
			currentLangCode: LangCodeEnUs,
		}
		json, _ := resource.GetLangJson(util.NewTraceContext(), string(LangCodeEnUs))
		managerInstance.enUsLangJson = string(json)
	})
	return managerInstance
}

func (m *Manager) UpdateLang(ctx context.Context, langCode LangCode) error {
	if !IsSupportedLangCode(string(langCode)) {
		return fmt.Errorf("unsupported lang code: %s", langCode)
	}

	json, err := m.GetLangJson(ctx, langCode)
	if err != nil {
		return err
	}

	m.currentLangCode = langCode
	m.currentLangJson = json
	m.pluginLangJsonMap.Clear()
	return nil
}

func (m *Manager) GetLangJson(ctx context.Context, langCode LangCode) (string, error) {
	json, err := resource.GetLangJson(ctx, string(langCode))
	if err != nil {
		return "", err
	}

	return string(json), nil
}

func (m *Manager) TranslateWox(ctx context.Context, key string) string {
	originKey := key

	if strings.HasPrefix(key, "i18n:") {
		key = key[5:]
	}

	result := gjson.Get(m.currentLangJson, key)
	if result.Exists() {
		return result.String()
	}

	enUsResult := gjson.Get(m.enUsLangJson, key)
	if enUsResult.Exists() {
		return enUsResult.String()
	}

	return originKey
}

func (m *Manager) TranslateWoxEnUs(ctx context.Context, key string) string {
	originKey := key

	if strings.HasPrefix(key, "i18n:") {
		key = key[5:]
	}

	enUsResult := gjson.Get(m.enUsLangJson, key)
	if enUsResult.Exists() {
		return enUsResult.String()
	}

	return originKey
}

func (m *Manager) TranslatePlugin(ctx context.Context, key string, pluginDirectory string) string {
	cacheKey := fmt.Sprintf("%s:%s", pluginDirectory, m.currentLangCode)
	if v, ok := m.pluginLangJsonMap.Load(cacheKey); ok {
		return m.translatePluginFromJson(ctx, key, v)
	}

	jsonPath := path.Join(pluginDirectory, "lang", fmt.Sprintf("%s.json", m.currentLangCode))
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		return key
	}

	json, err := os.ReadFile(jsonPath)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("error reading lang file(%s): %s", jsonPath, err.Error()))
		return key
	}

	m.pluginLangJsonMap.Store(cacheKey, string(json))
	return m.translatePluginFromJson(ctx, key, string(json))
}

func (m *Manager) translatePluginFromJson(ctx context.Context, key string, langJson string) string {
	originKey := key

	if strings.HasPrefix(key, "i18n:") {
		key = key[5:]
	}

	result := gjson.Get(langJson, key)
	if result.Exists() {
		return result.String()
	}

	return originKey
}
