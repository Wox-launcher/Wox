package i18n

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"wox/resource"
	"wox/util"

	"github.com/tidwall/gjson"
)

var managerInstance *Manager
var managerOnce sync.Once

type Manager struct {
	currentLangCode  LangCode
	enUsLangJson     string
	currentLangJson  string
	currentLangCache *util.HashMap[string, string]
	enUsCache        *util.HashMap[string, string]
}

func GetI18nManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{
			currentLangCode:  LangCodeEnUs,
			currentLangCache: util.NewHashMap[string, string](),
			enUsCache:        util.NewHashMap[string, string](),
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
	m.currentLangCache.Clear()
	return nil
}

func (m *Manager) GetCurrentLangCode() LangCode {
	return m.currentLangCode
}

func (m *Manager) GetLangJson(ctx context.Context, langCode LangCode) (string, error) {
	json, err := resource.GetLangJson(ctx, string(langCode))
	if err != nil {
		return "", err
	}

	return string(json), nil
}

// TranslateWox translates a key using the current language json file.
// Because this function is hot path, we use cache to improve performance
func (m *Manager) TranslateWox(ctx context.Context, key string) string {
	originKey := key

	key = strings.TrimPrefix(key, "i18n:")
	if value, ok := m.currentLangCache.Load(key); ok {
		return value
	}
	result := gjson.Get(m.currentLangJson, key)
	if result.Exists() {
		value := result.String()
		m.currentLangCache.Store(key, value)
		return value
	}

	// fallback to en_US
	if value, ok := m.enUsCache.Load(key); ok {
		return value
	}
	enUsResult := gjson.Get(m.enUsLangJson, key)
	if enUsResult.Exists() {
		value := enUsResult.String()
		m.enUsCache.Store(key, value)
		return value
	}

	return originKey
}

func (m *Manager) TranslateWoxEnUs(ctx context.Context, key string) string {
	originKey := key

	key = strings.TrimPrefix(key, "i18n:")
	if value, ok := m.enUsCache.Load(key); ok {
		return value
	}
	enUsResult := gjson.Get(m.enUsLangJson, key)
	if enUsResult.Exists() {
		value := enUsResult.String()
		m.enUsCache.Store(key, value)
		return value
	}

	return originKey
}

// TranslateI18nMap translates a key using metadata i18n map that may include both inline and lang file values.
// Priority:
// 1. I18n map for current language
// 2. I18n map for en_US fallback
// 3. Return original key
func (m *Manager) TranslateI18nMap(_ context.Context, key string, pluginI18n map[string]map[string]string) string {
	originKey := key

	key = strings.TrimPrefix(key, "i18n:")

	// 1. Try current language
	if translated := m.translateFromInlineI18n(key, string(m.currentLangCode), pluginI18n); translated != "" {
		return translated
	}

	// 2. Try en_US fallback
	if m.currentLangCode != LangCodeEnUs {
		if translated := m.translateFromInlineI18n(key, string(LangCodeEnUs), pluginI18n); translated != "" {
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
