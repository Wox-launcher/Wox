package launcher

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"wox/i18n"
	"wox/ui/contract"
)

type languageServices interface {
	GeneralSettings(context.Context, string) (contract.GeneralSettings, error)
	LanguageBundle(context.Context, string, i18n.LangCode) (map[string]string, error)
}

// languageCatalog publishes immutable translation tables shared by instance surfaces.
type languageCatalog struct {
	translationsMu       sync.RWMutex
	translationsLanguage string
	translations         map[string]string
	translationsRevision atomic.Uint64
}

// reload resolves fallback translations before atomically publishing a new table.
func (c *languageCatalog) reload(ctx context.Context, services languageServices, sessionID string) (bool, error) {
	settings, err := services.GeneralSettings(ctx, sessionID)
	if err != nil {
		return false, fmt.Errorf("load language setting: %w", err)
	}
	langCode := settings.LangCode
	if langCode == "" {
		langCode = i18n.LangCodeEnUs
	}
	// The bundle is core's own parsed table and is read-only here. English reuses it
	// directly instead of holding a second copy of the whole language pack; other
	// languages need a merged copy so missing keys fall back to English.
	translations, err := services.LanguageBundle(ctx, sessionID, langCode)
	if err != nil {
		return false, fmt.Errorf("load language bundle: %w", err)
	}
	if langCode != i18n.LangCodeEnUs {
		fallback, englishErr := services.LanguageBundle(ctx, sessionID, i18n.LangCodeEnUs)
		if englishErr != nil {
			return false, fmt.Errorf("load fallback language bundle: %w", englishErr)
		}
		merged := make(map[string]string, len(fallback))
		for key, value := range fallback {
			merged[key] = value
		}
		for key, value := range translations {
			if value != "" {
				merged[key] = value
			}
		}
		translations = merged
	}
	c.translationsMu.Lock()
	languageChanged := c.translationsLanguage != "" && c.translationsLanguage != string(langCode)
	c.translationsLanguage = string(langCode)
	c.translations = translations
	c.translationsMu.Unlock()
	c.translationsRevision.Add(1)
	return languageChanged, nil
}
