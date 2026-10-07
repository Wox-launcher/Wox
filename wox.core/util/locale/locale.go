package locale

import (
	"os"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/text/language"
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

// GetCurrencyRegion uses Linux monetary preferences or the existing OS locale lookup.
func GetCurrencyRegion() string {
	if runtime.GOOS == "linux" {
		return currencyRegionFromLocales(os.Getenv("LC_ALL"), os.Getenv("LC_MONETARY"), os.Getenv("LANG"))
	}
	_, region := GetLocale()
	return region
}

// currencyRegionFromLocales follows Linux monetary locale priority without inferring a missing region.
func currencyRegionFromLocales(all, monetary, lang string) string {
	for _, name := range []string{all, monetary, lang} {
		if name == "" {
			continue
		}
		name, _, _ = strings.Cut(name, ".")
		name, _, _ = strings.Cut(name, "@")
		tag, err := language.Parse(strings.ReplaceAll(name, "_", "-"))
		if err != nil {
			return ""
		}
		_, _, region := tag.Raw()
		if region.String() == "ZZ" {
			return ""
		}
		return region.String()
	}
	return ""
}
