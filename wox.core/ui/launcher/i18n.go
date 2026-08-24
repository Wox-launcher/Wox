package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"wox/i18n"
)

// reloadTranslations loads the flat language bundle embedded by core.
func (a *App) reloadTranslations() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	settings, err := a.services.GeneralSettings(ctx, a.sessionID)
	if err != nil {
		return fmt.Errorf("load language setting: %w", err)
	}
	langCode := settings.LangCode
	if langCode == "" {
		langCode = i18n.LangCodeEnUs
	}
	encoded, err := a.services.LanguageJSON(ctx, a.sessionID, langCode)
	if err != nil {
		return fmt.Errorf("load language bundle: %w", err)
	}
	translations := map[string]string{}
	if err := json.Unmarshal([]byte(encoded), &translations); err != nil {
		return fmt.Errorf("decode language bundle: %w", err)
	}
	if langCode != i18n.LangCodeEnUs {
		englishJSON, englishErr := a.services.LanguageJSON(ctx, a.sessionID, i18n.LangCodeEnUs)
		if englishErr != nil {
			return fmt.Errorf("load fallback language bundle: %w", englishErr)
		}
		fallback := map[string]string{}
		if err := json.Unmarshal([]byte(englishJSON), &fallback); err != nil {
			return fmt.Errorf("decode fallback language bundle: %w", err)
		}
		for key, value := range fallback {
			if translations[key] == "" {
				translations[key] = value
			}
		}
	}
	a.translationsMu.Lock()
	a.translations = translations
	a.translationsMu.Unlock()
	a.translationsRevision.Add(1)
	a.invalidateAllWindows()
	return nil
}

func (a *App) translate(value string) string {
	if !strings.HasPrefix(value, "i18n:") {
		return value
	}
	key := strings.TrimPrefix(value, "i18n:")
	a.translationsMu.RLock()
	translated := a.translations[key]
	a.translationsMu.RUnlock()
	if translated != "" {
		return translated
	}
	return strings.ReplaceAll(key, "_", " ")
}

// translationSnapshot isolates matching and rendering from concurrent language reloads.
func (a *App) translationSnapshot() map[string]string {
	a.translationsMu.RLock()
	defer a.translationsMu.RUnlock()
	snapshot := make(map[string]string, len(a.translations))
	for key, value := range a.translations {
		snapshot[key] = value
	}
	return snapshot
}
