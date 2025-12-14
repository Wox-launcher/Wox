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

// TranslatePlugin translates a key using inline i18n config from plugin.json or lang files.
// Priority:
// 1. Inline i18n config for current language
// 2. Lang file for current language (lang/{langCode}.json)
// 3. Inline i18n config for en_US (fallback)
// 4. Lang file for en_US (fallback)
// 5. Return original key
func (m *Manager) TranslatePlugin(ctx context.Context, key string, pluginDirectory string, inlineI18n map[string]map[string]string) string {
	originKey := key

	if strings.HasPrefix(key, "i18n:") {
		key = key[5:]
	}

	// 1. Try inline i18n for current language
	if translated := m.translateFromInlineI18n(key, string(m.currentLangCode), inlineI18n); translated != "" {
		return translated
	}

	// 2. Try lang file for current language
	if translated := m.translateFromLangFile(ctx, key, pluginDirectory, string(m.currentLangCode)); translated != "" {
		return translated
	}

	// 3. Try inline i18n for en_US (fallback)
	if m.currentLangCode != LangCodeEnUs {
		if translated := m.translateFromInlineI18n(key, string(LangCodeEnUs), inlineI18n); translated != "" {
			return translated
		}

		// 4. Try lang file for en_US (fallback)
		if translated := m.translateFromLangFile(ctx, key, pluginDirectory, string(LangCodeEnUs)); translated != "" {
			return translated
		}
	}

	return originKey
}

// translateFromInlineI18n looks up a key in the inline i18n map
func (m *Manager) translateFromInlineI18n(key string, langCode string, inlineI18n map[string]map[string]string) string {
	if inlineI18n == nil {
		return ""
	}
	if langMap, ok := inlineI18n[langCode]; ok {
		if translated, ok := langMap[key]; ok {
			return translated
		}
	}
	return ""
}

// translateFromLangFile looks up a key in the lang file (with caching)
func (m *Manager) translateFromLangFile(ctx context.Context, key string, pluginDirectory string, langCode string) string {
	if pluginDirectory == "" {
		return ""
	}

	cacheKey := fmt.Sprintf("%s:%s", pluginDirectory, langCode)
	if v, ok := m.pluginLangJsonMap.Load(cacheKey); ok {
		result := gjson.Get(v, key)
		if result.Exists() {
			return result.String()
		}
		return ""
	}

	jsonPath := path.Join(pluginDirectory, "lang", fmt.Sprintf("%s.json", langCode))
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		// Cache empty string to avoid repeated file checks
		m.pluginLangJsonMap.Store(cacheKey, "")
		return ""
	}

	jsonContent, err := os.ReadFile(jsonPath)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("error reading lang file(%s): %s", jsonPath, err.Error()))
		m.pluginLangJsonMap.Store(cacheKey, "")
		return ""
	}

	m.pluginLangJsonMap.Store(cacheKey, string(jsonContent))
	result := gjson.Get(string(jsonContent), key)
	if result.Exists() {
		return result.String()
	}
	return ""
}
