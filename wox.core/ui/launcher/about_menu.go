package launcher

import (
	"context"
	"fmt"
	"strings"

	"wox/common"
	"wox/common/icons"
	woxui "wox/ui/runtime"
	"wox/util"
)

const (
	aboutMenuFeedbackID    = "about-feedback"
	aboutMenuGuideID       = "about-guide"
	aboutMenuChangelogID   = "about-changelog"
	aboutMenuSettingsID    = "about-settings"
	aboutMenuPluginStoreID = "about-plugin-store"
	aboutMenuCommunityID   = "about-community"
	aboutMenuGithubID      = "about-github"
	aboutMenuRedditID      = "about-reddit"
	aboutMenuDiscordID     = "about-discord"

	aboutMenuFeedbackQuery    = "feedback "
	aboutMenuPluginStoreQuery = "wpm install "
	aboutMenuGuideURL         = "https://www.woxlauncher.com/guide/introduction.html"
	aboutMenuGuideURLZh       = "https://www.woxlauncher.com/zh/guide/introduction.html"
	aboutMenuChangelogURL     = "https://github.com/Wox-launcher/Wox/releases"
	aboutMenuGithubURL        = "https://github.com/Wox-launcher/Wox"
	aboutMenuRedditURL        = "https://www.reddit.com/r/WoxLauncher/"
	aboutMenuDiscordURL       = "https://discord.gg/NnahFAwm3"

	aboutMenuDefaultSettingsPath = "/"
	aboutMenuGuideTail           = "woxlauncher.com"
	aboutMenuGithubTail          = "Wox-Launcher/Wox"
	aboutMenuRedditTail          = "r/WoxLauncher"
	// aboutMenuDiscordUserCount is the advertised Discord size. Raise it by hand as the community grows.
	aboutMenuDiscordUserCount = 3

	aboutMenuTooltipName = "go-ui-about-menu"
)

// aboutMenuHotkey is the launcher-local shortcut: primary+shift+k.
func aboutMenuHotkey() string {
	return primaryHotkey("shift+k")
}

// aboutMenuVersionTail formats the running version for the Changelog row.
func aboutMenuVersionTail(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(version), "v") {
		return version
	}
	return "v" + version
}

// aboutMenuDiscordTail formats the member-count tail from aboutMenuDiscordUserCount.
func aboutMenuDiscordTail(template string) string {
	if !strings.Contains(template, "%d") {
		template = "%d+ users"
	}
	return fmt.Sprintf(template, aboutMenuDiscordUserCount)
}

func aboutMenuEntries(version string) []actionPanelEntry {
	local := actionPanelSourceLocal
	return []actionPanelEntry{
		{Key: "about:feedback", ID: aboutMenuFeedbackID, Name: "i18n:ui_about_menu_feedback", Icon: fromCoreImage(icons.Get(icons.ActionFeedback)), TailIcon: fromCoreImage(icons.Get(icons.PluginFeedback)), Source: local},
		{Key: "about:guide", ID: aboutMenuGuideID, Name: "i18n:ui_about_menu_guide", Icon: settingControlIconSource("documentation"), Tail: aboutMenuGuideTail, Source: local},
		{Key: "about:changelog", ID: aboutMenuChangelogID, Name: "i18n:ui_about_menu_changelog", Icon: settingControlIconSource("article"), Tail: aboutMenuVersionTail(version), Source: local},
		{Key: "about:settings", ID: aboutMenuSettingsID, Name: "i18n:ui_about_menu_settings", Icon: fromCoreImage(icons.Get(icons.ActionSettings)), TailIcon: fromCoreImage(icons.Get(icons.BrandWox)), Source: local},
		{Key: "about:plugin-store", ID: aboutMenuPluginStoreID, Name: "i18n:ui_about_menu_plugin_store", Icon: settingNavIconSource("plugins.installed"), TailIcon: fromCoreImage(icons.Get(icons.PluginWPM)), Source: local},
		{Key: "about:community", ID: aboutMenuCommunityID, Name: "i18n:ui_about_menu_community", IsGroupHeader: true, Source: local},
		{Key: "about:github", ID: aboutMenuGithubID, Name: "i18n:ui_about_menu_github", Icon: fromCoreImage(icons.Get(icons.BrandGithubMonochrome)), Tail: aboutMenuGithubTail, Source: local},
		{Key: "about:reddit", ID: aboutMenuRedditID, Name: "i18n:ui_about_menu_reddit", Icon: fromCoreImage(icons.Get(icons.BrandRedditMonochrome)), Tail: aboutMenuRedditTail, Source: local},
		{Key: "about:discord", ID: aboutMenuDiscordID, Name: "i18n:ui_about_menu_discord", Icon: fromCoreImage(icons.Get(icons.BrandDiscordMonochrome)), Tail: "i18n:ui_about_menu_discord_tail", Source: local},
	}
}

// aboutMenuQuery is the launcher query that opens a system plugin in place.
func isAboutMenuCommunityEntry(entry actionPanelEntry) bool {
	return entry.ID == aboutMenuGithubID || entry.ID == aboutMenuRedditID || entry.ID == aboutMenuDiscordID
}

func aboutMenuQuery(id string) (string, bool) {
	switch id {
	case aboutMenuFeedbackID:
		return aboutMenuFeedbackQuery, true
	case aboutMenuPluginStoreID:
		return aboutMenuPluginStoreQuery, true
	default:
		return "", false
	}
}

// aboutMenuGuideURLForLang uses the Chinese docs only for zh locales.
func aboutMenuGuideURLForLang(lang string) string {
	if isChineseLangCode(lang) {
		return aboutMenuGuideURLZh
	}
	return aboutMenuGuideURL
}

func isChineseLangCode(lang string) bool {
	code := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(lang), "-", "_"))
	return code == "zh_cn" || code == "zh" || strings.HasPrefix(code, "zh_")
}

func aboutMenuExternalURL(id, lang string) (string, bool) {
	switch id {
	case aboutMenuGuideID:
		return aboutMenuGuideURLForLang(lang), true
	case aboutMenuChangelogID:
		return aboutMenuChangelogURL, true
	case aboutMenuGithubID:
		return aboutMenuGithubURL, true
	case aboutMenuRedditID:
		return aboutMenuRedditURL, true
	case aboutMenuDiscordID:
		return aboutMenuDiscordURL, true
	default:
		return "", false
	}
}

func aboutMenuSettingsRoute(id string) (string, bool) {
	if id == aboutMenuSettingsID {
		return aboutMenuDefaultSettingsPath, true
	}
	return "", false
}

// aboutMenuButtonVisible keeps the toolbar entry while the menu is open even if a message arrives.
func aboutMenuButtonVisible(panelOpen bool, purpose actionPanelPurpose, message *toolbarMessage) bool {
	if panelOpen && purpose == actionPanelPurposeAbout {
		return true
	}
	return message == nil
}

// currentLangCode reads the live General setting, then the loaded translation bundle.
func (a *App) currentLangCode() string {
	if a.generalSettings != nil {
		if lang := strings.TrimSpace(a.generalSettings.Data().LangCode); lang != "" {
			return lang
		}
	}
	return a.translationsLanguage
}

// runningVersion reads the already-known core version without opening Settings.
func (a *App) runningVersion() string {
	if a.aboutSettings != nil {
		if version := strings.TrimSpace(a.aboutSettings.Snapshot().Version); version != "" {
			return version
		}
	}
	if a.services != nil {
		version, err := a.services.Version(context.Background(), a.sessionID)
		if err == nil {
			return strings.TrimSpace(version)
		}
	}
	return ""
}

// toggleAboutMenu opens or closes the left About Menu without using plugin results.
func (a *App) toggleAboutMenu() {
	if a.actionPanel && a.actionPanelPurpose == actionPanelPurposeAbout {
		a.hideActionPanel()
		return
	}
	a.openActionPanel(actionPanelPurposeAbout)
}

// activateAboutMenuEntry dispatches a stable menu ID without constructing plugin results.
func (a *App) activateAboutMenuEntry(entry actionPanelEntry) bool {
	if query, ok := aboutMenuQuery(entry.ID); ok {
		a.openAboutMenuQuery(query)
		return true
	}
	if path, ok := aboutMenuSettingsRoute(entry.ID); ok {
		a.openAboutMenuSettings(path)
		return true
	}
	if target, ok := aboutMenuExternalURL(entry.ID, a.currentLangCode()); ok {
		a.openAboutMenuURL(target)
		return true
	}
	return false
}

// openAboutMenuQuery replaces the launcher query so a system plugin can take over.
func (a *App) openAboutMenuQuery(text string) {
	if err := a.ChangeQuery(a.lifecycleCtx, common.PlainQuery{QueryType: "input", QueryText: text}); err != nil {
		util.GetLogger().Error(a.lifecycleCtx, fmt.Sprintf("change query from about menu: %v", err))
		if a.visible {
			a.openActionPanel(actionPanelPurposeAbout)
		}
		a.notifyAboutMenuError("i18n:ui_about_menu_open_link_failed")
	}
}

// openAboutMenuURL opens one confirmed URL in the system browser and hides the launcher on success.
func (a *App) openAboutMenuURL(target string) {
	if a.window == nil {
		return
	}
	if err := a.window.OpenExternalURL(target); err != nil {
		util.GetLogger().Error(a.lifecycleCtx, fmt.Sprintf("open about menu url %s: %v", target, err))
		a.notifyAboutMenuError("i18n:ui_about_menu_open_link_failed")
		return
	}
	a.hideActionPanel()
	if err := a.hideWindow(true); err != nil {
		util.GetLogger().Error(a.lifecycleCtx, fmt.Sprintf("hide launcher after about menu url: %v", err))
	}
}

// openAboutMenuSettings reuses the existing Settings window so About and Settings do not steal query focus.
func (a *App) openAboutMenuSettings(path string) {
	// Reset without restoring query focus so Settings can become the key window.
	a.resetActionPanelLocked()
	if a.window != nil {
		_ = a.window.Invalidate()
	}
	if err := a.OpenSetting(context.Background(), common.SettingWindowContext{Path: path}); err != nil {
		util.GetLogger().Error(a.lifecycleCtx, fmt.Sprintf("open about menu settings %s: %v", path, err))
		if a.visible {
			a.openActionPanel(actionPanelPurposeAbout)
		}
		a.notifyAboutMenuError("i18n:ui_about_menu_open_settings_failed")
	}
}

// notifyAboutMenuError displays failures inside the menu; toolbar messages stay deferred.
func (a *App) notifyAboutMenuError(key string) {
	a.aboutMenuError = a.translate(key)
	if a.window != nil {
		_ = a.window.Invalidate()
	}
}

// dismissAboutMenuTooltip clears the button hint so it cannot float over the open panel.
func (a *App) dismissAboutMenuTooltip() {
	a.aboutMenuTooltipRevision.Add(1)
	a.hideNativeHoverTooltip(aboutMenuTooltipName, "hide about menu tooltip")
}

// onAboutMenuHover shows the About Menu title and keycaps after the shared hover dwell.
func (a *App) onAboutMenuHover(inside bool, bounds woxui.Rect) {
	if a.actionPanel && a.actionPanelPurpose == actionPanelPurposeAbout {
		a.dismissAboutMenuTooltip()
		return
	}
	a.setNativeHoverTooltipWithHotkeys(
		&a.aboutMenuTooltipRevision, aboutMenuTooltipName, "update about menu tooltip",
		inside, a.translate("i18n:toolbar_about_menu"), formatHotkeyLabels(aboutMenuHotkey()), bounds, "top",
		func() *woxui.Window { return a.window },
	)
}
