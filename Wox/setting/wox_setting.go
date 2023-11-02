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
	MainHotkey           PlatformSettingValue[string]
	UsePinYin            bool
	SwitchInputMethodABC bool
	ShowTray             bool
	LangCode             i18n.LangCode
	QueryHotkeys         PlatformSettingValue[[]QueryHotkey]
	LastQueryMode        LastQueryMode
}

type LastQueryMode = string

const (
	LastQueryModePreserve LastQueryMode = "preserve" // preserve last query and select all for quick modify
	LastQueryModeEmpty    LastQueryMode = "empty"    // empty last query
)

type QueryHotkey struct {
	Hotkey string
	Query  string
}

func GetDefaultWoxSetting(ctx context.Context) WoxSetting {
	usePinYin := false
	langCode := i18n.LangCodeEnUs
	switchInputMethodABC := false
	if isZhCN() {
		usePinYin = true
		switchInputMethodABC = true
		langCode = i18n.LangCodeZhCn
	}

	return WoxSetting{
		MainHotkey: PlatformSettingValue[string]{
			WinValue:   "alt+space",
			MacValue:   "command+space",
			LinuxValue: "alt+space",
		},
		UsePinYin:            usePinYin,
		SwitchInputMethodABC: switchInputMethodABC,
		ShowTray:             true,
		LangCode:             langCode,
		LastQueryMode:        LastQueryModeEmpty,
	}
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
