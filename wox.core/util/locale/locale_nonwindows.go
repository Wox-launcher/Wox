//go:build !windows

package locale

import (
	"os"
	"runtime"
	"strings"
	"wox/util/shell"
)

// detectLocale preserves the platform locale lookup and its English fallback.
func detectLocale() (string, string) {
	osHost := runtime.GOOS
	defaultLang := "en"
	defaultLoc := "US"
	switch osHost {
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
