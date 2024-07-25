package dto

type SettingTheme struct {
	ThemeId        string
	ThemeName      string
	ThemeAuthor    string
	ThemeUrl       string
	Version        string
	Description    string
	IsInstalled    bool
	IsSystem       bool
	IsUpgradable   bool
	ScreenshotUrls []string
}
