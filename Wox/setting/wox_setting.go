package setting

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"wox/i18n"
)

type WoxSetting struct {
	MainHotkey           string
	UsePinYin            bool
	SwitchInputMethodABC bool
	ShowTray             bool
	LangCode             i18n.LangCode
	QueryHotkeys         []QueryHotkey
}

type QueryHotkey struct {
	Hotkey string
	Query  string
}

func GetDefaultWoxSetting(ctx context.Context) WoxSetting {
	usePinYin := false
	langCode := i18n.LangCodeEnUs
	if isZhCN() {
		usePinYin = true
		langCode = i18n.LangCodeZhCn
	}

	return WoxSetting{
		MainHotkey: getDefaultMainHotkey(ctx),
		UsePinYin:  usePinYin,
		ShowTray:   true,
		LangCode:   langCode,
	}
}

func getDefaultMainHotkey(ctx context.Context) string {
	combineKey := "alt+space"
	if strings.ToLower(runtime.GOOS) == "darwin" {
		combineKey = "command+space"
	}
	return combineKey
}

func isZhCN() bool {
	lang, locale := getLocale()
	return strings.ToLower(lang) == "zh" && strings.ToLower(locale) == "cn"
}

func getLocale() (string, string) {
	osHost := runtime.GOOS
	defaultLang := "en"
	defaultLoc := "US"
	switch osHost {
	case "windows":
		// Exec powershell Get-Culture on Windows.
		cmd := exec.Command("powershell", "Get-Culture | select -exp Name")
		output, err := cmd.Output()
		if err == nil {
			langLocRaw := strings.TrimSpace(string(output))
			langLoc := strings.Split(langLocRaw, "-")
			lang := langLoc[0]
			loc := langLoc[1]
			return lang, loc
		}
	case "darwin":
		// Exec shell Get-Culture on MacOS.
		cmd := exec.Command("sh", "osascript -e 'user locale of (get system info)'")
		output, err := cmd.Output()
		if err == nil {
			langLocRaw := strings.TrimSpace(string(output))
			langLoc := strings.Split(langLocRaw, "_")
			lang := langLoc[0]
			loc := langLoc[1]
			return lang, loc
		}
	case "linux":
		envlang, ok := os.LookupEnv("LANG")
		if ok {
			langLocRaw := strings.TrimSpace(envlang)
			langLocRaw = strings.Split(envlang, ".")[0]
			langLoc := strings.Split(langLocRaw, "_")
			lang := langLoc[0]
			loc := langLoc[1]
			return lang, loc
		}
	}
	return defaultLang, defaultLoc
}
