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
	currentLangCode LangCode
	enUsLangJson    string
	currentLangJson string
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

	key = strings.TrimPrefix(key, "i18n:")
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

	key = strings.TrimPrefix(key, "i18n:")
	enUsResult := gjson.Get(m.enUsLangJson, key)
	if enUsResult.Exists() {
		return enUsResult.String()
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
