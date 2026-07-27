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
	a.mu.Lock()
	a.translations = translations
	a.mu.Unlock()
	a.invalidateAllWindows()
	return nil
}

func (a *App) translate(value string) string {
	if !strings.HasPrefix(value, "i18n:") {
		return value
	}
	key := strings.TrimPrefix(value, "i18n:")
	a.mu.RLock()
	translated := a.translations[key]
	a.mu.RUnlock()
	if translated != "" {
		return translated
	}
	return strings.ReplaceAll(key, "_", " ")
}
