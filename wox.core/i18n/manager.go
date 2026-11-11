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
	revision        uint64
	packs           *packStore
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
	pack, err := m.ReadPack(ctx, langCode)
	if err == nil {
		m.ActivatePack(pack)
	}
	return err
}

// ReadPack loads only bundled or cached translations. Network access is explicit.
func (m *Manager) ReadPack(ctx context.Context, langCode LangCode) (Pack, error) {
	if !IsSupportedLangCode(string(langCode)) {
		return Pack{}, fmt.Errorf("unsupported lang code: %s", langCode)
	}
	if !IsBundled(langCode) {
		if m.packs == nil {
			return Pack{}, fmt.Errorf("language pack cache is unavailable")
		}
		return m.packs.read(langCode)
	}
	if langCode == LangCodeEnUs && m.enUsLang != nil {
		return Pack{code: langCode, translations: m.enUsLang}, nil
	}
	lang, err := loadLangMap(ctx, langCode)
	return Pack{code: langCode, translations: lang}, err
}

// ActivatePack publishes the already validated table and invalidates translation caches.
func (m *Manager) ActivatePack(pack Pack) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activatePackLocked(pack)
}

func (m *Manager) activatePackLocked(pack Pack) {
	m.currentLangCode = pack.code
	m.currentLang = pack.translations
	m.revision++
}

// RefreshActivePack reloads updated translations only while this pack is the active language.
func (m *Manager) RefreshActivePack(ctx context.Context, code LangCode) error {
	if m.GetCurrentLangCode() != code {
		return nil
	}
	pack, err := m.ReadPack(ctx, code)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentLangCode == code {
		m.activatePackLocked(pack)
	}
	return nil
}

func (m *Manager) GetCurrentLangCode() LangCode {
	code, _ := m.ActiveLanguage()
	return code
}

// ActiveLanguage returns the code and revision from the same published snapshot.
func (m *Manager) ActiveLanguage() (LangCode, uint64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentLangCode, m.revision
}

// GetLangMap returns the parsed table for langCode. The English and current-language tables
// are the manager's own maps so the UI can share them instead of parsing a second copy; they
// are never mutated after load and callers must treat them as read-only.
func (m *Manager) GetLangMap(ctx context.Context, langCode LangCode) (map[string]string, error) {
	m.mu.RLock()
	currentLangCode, currentLang, enUsLang := m.currentLangCode, m.currentLang, m.enUsLang
	m.mu.RUnlock()
	switch langCode {
	case LangCodeEnUs:
		return enUsLang, nil
	case currentLangCode:
		return currentLang, nil
	}
	pack, err := m.ReadPack(ctx, langCode)
	return pack.translations, err
}

func (m *Manager) GetLangJson(ctx context.Context, langCode LangCode) (string, error) {
	if !IsBundled(langCode) {
		pack, err := m.ReadPack(ctx, langCode)
		if err != nil {
			return "", err
		}
		data, err := json.Marshal(pack.translations)
		return string(data), err
	}
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
