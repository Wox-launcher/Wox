package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// reloadTranslations loads the flat language bundle embedded by core.
func (a *App) reloadTranslations() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var languageSetting struct {
		LangCode string `json:"LangCode"`
	}
	if err := a.client.Post(ctx, "/setting/wox", map[string]any{}, &languageSetting); err != nil {
		return fmt.Errorf("load language setting: %w", err)
	}
	if languageSetting.LangCode == "" {
		languageSetting.LangCode = "en_US"
	}
	var encoded string
	if err := a.client.Post(ctx, "/lang/json", map[string]string{"langCode": languageSetting.LangCode}, &encoded); err != nil {
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
