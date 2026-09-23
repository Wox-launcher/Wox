package launcher

import (
	"context"
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
	// The bundle is core's own parsed table and is read-only here. English reuses it
	// directly instead of holding a second copy of the whole language pack; other
	// languages need a merged copy so missing keys fall back to English.
	translations, err := a.services.LanguageBundle(ctx, a.sessionID, langCode)
	if err != nil {
		return fmt.Errorf("load language bundle: %w", err)
	}
	if langCode != i18n.LangCodeEnUs {
		fallback, englishErr := a.services.LanguageBundle(ctx, a.sessionID, i18n.LangCodeEnUs)
		if englishErr != nil {
			return fmt.Errorf("load fallback language bundle: %w", englishErr)
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
	a.translationsMu.Lock()
	languageChanged := a.translationsLanguage != "" && a.translationsLanguage != string(langCode)
	a.translationsLanguage = string(langCode)
	a.translations = translations
	a.translationsMu.Unlock()
	a.translationsRevision.Add(1)
	if languageChanged {
		var reload, store bool
		if err := a.runOnUI("invalidate translated plugin catalogs", func() {
			// Catalog entries contain already translated metadata, unlike ordinary UI labels.
			a.pluginSettings.memoryRevision++
			a.pluginSettings.invalidateCachedPlugins(false)
			a.pluginSettings.invalidateCachedPlugins(true)
			a.pluginSettings.SetPluginsLoaded(false)
			a.pluginSettings.SetPluginsLoading(false)
			a.settingsSearch.revision++
			a.settingsSearch.SetPlugins(nil)
			a.settingsSearch.SetLoaded(false)
			a.settingsSearch.SetLoading(false)
			a.reloadChatResourceName("mentions")
			reload = a.settingsOpen && a.settingTab == "plugins"
			store = a.pluginSettings.PluginsStore()
		}); err != nil {
			return err
		}
		if reload {
			if err := a.reloadPlugins(store, ""); err != nil {
				return err
			}
		}
	}
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

// translationSnapshot returns the current table for matching and rendering. Reloads publish a
// new map instead of mutating the old one, so handing out the reference is safe and avoids
// copying the whole language pack on every action panel keystroke.
func (a *App) translationSnapshot() map[string]string {
	a.translationsMu.RLock()
	defer a.translationsMu.RUnlock()
	return a.translations
}
