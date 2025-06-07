package locale

import (
	"os"
	"runtime"
	"strings"
	"wox/util/shell"
)

func IsZhCN() bool {
	lang, locale := GetLocale()
	return strings.ToLower(lang) == "zh" && strings.ToLower(locale) == "cn"
}

// GetLocale returns the user's language and region
func GetLocale() (string, string) {
	osHost := runtime.GOOS
	defaultLang := "en"
	defaultLoc := "US"
	switch osHost {
	case "windows":
		// Exec powershell Get-Culture on Windows.
		output, err := shell.RunOutput("powershell", "Get-Culture | select -exp Name")
		if err == nil {
			langLocRaw := strings.TrimSpace(string(output))
			langLoc := strings.Split(langLocRaw, "-")
			if len(langLoc) >= 2 {
				lang := langLoc[0]
				loc := langLoc[1]
				return lang, loc
			}
		}
	case "darwin":
		// Exec shell Get-Culture on MacOS.
		output, err := shell.RunOutput("osascript", "-e", "user locale of (get system info)")
		if err == nil {
			langLocRaw := strings.TrimSpace(string(output))
			langLoc := strings.Split(langLocRaw, "_")
			if len(langLoc) >= 2 {
				lang := langLoc[0]
				loc := langLoc[1]
				return lang, loc
			}
		}
	case "linux":
		envlang, ok := os.LookupEnv("LANG")
		if ok {
			langLocRaw := strings.TrimSpace(envlang)
			langLocRaw = strings.Split(envlang, ".")[0]
			langLoc := strings.Split(langLocRaw, "_")
			if len(langLoc) >= 2 {
				lang := langLoc[0]
				loc := langLoc[1]
				return lang, loc
			}
		}
	}
	return defaultLang, defaultLoc
}
