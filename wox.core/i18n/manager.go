package i18n

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"wox/resource"
	"wox/util"
)

var managerInstance *Manager
var managerOnce sync.Once

type Manager struct {
	mu              sync.RWMutex
	currentLangCode LangCode
	// Parsed language tables. The raw JSON is discarded after Unmarshal so the
	// process does not keep both the source bytes and the lookup map.
	enUsLang    map[string]string
	currentLang map[string]string
}

func GetI18nManager() *Manager {
	managerOnce.Do(func() {
		enUsLang, _ := loadLangMap(util.NewTraceContext(), LangCodeEnUs)
		managerInstance = &Manager{
			currentLangCode: LangCodeEnUs,
			enUsLang:        enUsLang,
			currentLang:     enUsLang,
		}
	})
	return managerInstance
}

func (m *Manager) UpdateLang(ctx context.Context, langCode LangCode) error {
	if !IsSupportedLangCode(string(langCode)) {
		return fmt.Errorf("unsupported lang code: %s", langCode)
	}

	if langCode == LangCodeEnUs {
		m.mu.Lock()
		m.currentLangCode = langCode
		m.currentLang = m.enUsLang
		m.mu.Unlock()
		return nil
	}

	lang, err := loadLangMap(ctx, langCode)
	if err != nil {
		return err
	}

	m.mu.Lock()
	m.currentLangCode = langCode
	m.currentLang = lang
	m.mu.Unlock()
	return nil
}

func (m *Manager) GetCurrentLangCode() LangCode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentLangCode
}

func (m *Manager) GetLangJson(ctx context.Context, langCode LangCode) (string, error) {
	jsonBytes, err := resource.GetLangJson(ctx, string(langCode))
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// TranslateWox translates a key using the current language table.
func (m *Manager) TranslateWox(ctx context.Context, key string) string {
	originKey := key
	key = strings.TrimPrefix(key, "i18n:")

	m.mu.RLock()
	currentLang := m.currentLang
	enUsLang := m.enUsLang
	m.mu.RUnlock()

	if value, ok := currentLang[key]; ok {
		return value
	}
	if value, ok := enUsLang[key]; ok {
		return value
	}
	return originKey
}

func (m *Manager) TranslateWoxEnUs(ctx context.Context, key string) string {
	originKey := key
	key = strings.TrimPrefix(key, "i18n:")

	m.mu.RLock()
	enUsLang := m.enUsLang
	m.mu.RUnlock()

	if value, ok := enUsLang[key]; ok {
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
	if translated := m.translateFromInlineI18n(key, string(m.GetCurrentLangCode()), pluginI18n); translated != "" {
		return translated
	}

	// 2. Try en_US fallback
	if m.GetCurrentLangCode() != LangCodeEnUs {
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

// loadLangMap parses one embedded language file and drops the source JSON.
func loadLangMap(ctx context.Context, langCode LangCode) (map[string]string, error) {
	data, err := resource.GetLangJson(ctx, string(langCode))
	if err != nil {
		return map[string]string{}, err
	}
	var translations map[string]string
	if err := json.Unmarshal(data, &translations); err != nil {
		return map[string]string{}, err
	}
	if translations == nil {
		return map[string]string{}, nil
	}
	return translations, nil
}
