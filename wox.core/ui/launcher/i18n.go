package launcher

import (
	"context"
	"strings"
	"time"
)

// reloadTranslations loads the flat language bundle embedded by core.
func (a *App) reloadTranslations() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	languageChanged, err := a.language.reload(ctx, a.services, a.sessionID)
	if err != nil {
		return err
	}
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
			a.chat.reloadChatResourceName("mentions")
			reload = a.settingsWindow.open && a.settingsWindow.tab == "plugins"
			store = a.pluginSettings.PluginsStore()
		}); err != nil {
			return err
		}
		if reload {
			if err := a.pluginSettings.reloadPlugins(store, ""); err != nil {
				return err
			}
		}
	}
	a.invalidateAllWindows()
	return nil
}

func (a *languageCatalog) translate(value string) string {
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
func (a *languageCatalog) translationSnapshot() map[string]string {
	a.translationsMu.RLock()
	defer a.translationsMu.RUnlock()
	return a.translations
}
