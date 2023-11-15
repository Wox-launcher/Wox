package setting

import (
	"context"
	"os"
	"regexp"
	"runtime"
	"strings"
	"wox/i18n"
	"wox/util"
)

type WoxSetting struct {
	MainHotkey           PlatformSettingValue[string]
	SelectionHotkey      PlatformSettingValue[string]
	UsePinYin            bool
	SwitchInputMethodABC bool
	HideOnStart          bool
	HideOnLostFocus      bool
	ShowTray             bool
	LangCode             i18n.LangCode
	QueryHotkeys         PlatformSettingValue[[]QueryHotkey]
	QueryShortcuts       []QueryShortcut
	LastQueryMode        LastQueryMode
	ThemeId              string
}

type LastQueryMode = string

const (
	LastQueryModePreserve LastQueryMode = "preserve" // preserve last query and select all for quick modify
	LastQueryModeEmpty    LastQueryMode = "empty"    // empty last query
)

type QueryShortcut struct {
	Shortcut string // support index placeholder, e.g. shortcut "wi" => "wpm install {0} to {1}", when user input "wi 1 2", the query will be "wpm install 1 to 2"
	Query    string
}

func (q *QueryShortcut) HasPlaceholder() bool {
	return strings.Contains(q.Query, "{0}")
}

func (q *QueryShortcut) PlaceholderCount() int {
	return len(regexp.MustCompile(`(?m){\d}`).FindAllString(q.Query, -1))
}

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
		HideOnLostFocus:      true,
		LangCode:             langCode,
		LastQueryMode:        LastQueryModeEmpty,
		ThemeId:              "53c1d0a4-ffc8-4d90-91dc-b408fb0b9a03",
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
		output, err := util.ShellRunOutput("powershell", "Get-Culture | select -exp Name")
		if err == nil {
			langLocRaw := strings.TrimSpace(string(output))
			langLoc := strings.Split(langLocRaw, "-")
			lang := langLoc[0]
			loc := langLoc[1]
			return lang, loc
		}
	case "darwin":
		// Exec shell Get-Culture on MacOS.
		output, err := util.ShellRunOutput("osascript", "-e", "user locale of (get system info)")
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
