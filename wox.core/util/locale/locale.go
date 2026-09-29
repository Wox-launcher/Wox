package locale

import (
	"strings"
	"sync"
)

var (
	cachedLang   string
	cachedRegion string
	localeOnce   sync.Once
)

func IsZhCN() bool {
	lang, locale := GetLocale()
	return strings.ToLower(lang) == "zh" && strings.ToLower(locale) == "cn"
}

// GetLocale returns the user's language and region
func GetLocale() (string, string) {
	localeOnce.Do(func() {
		cachedLang, cachedRegion = detectLocale()
	})
	return cachedLang, cachedRegion
}
