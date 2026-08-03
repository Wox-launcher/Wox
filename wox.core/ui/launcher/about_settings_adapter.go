package launcher

import (
	"context"
	"log"

	launcherview "wox/ui/launcher/view"
	woxwidget "wox/ui/widget"
)

// buildAboutSettingsPage maps core version state and external actions into the About view.
func (a *App) buildAboutSettingsPage(snapshot settingsSnapshot, width, height, imageScale float32) woxwidget.Widget {
	version := snapshot.about.Version
	if version == "" && snapshot.about.Loading {
		version = a.translate("i18n:ui_about_version") + "…"
	}
	if version == "" {
		version = a.translate("i18n:ui_about_version")
	}
	status := ""
	if snapshot.about.Error != "" {
		status = snapshot.about.Error
	}
	theme := snapshot.palette.componentTheme()
	iconTint := theme.ResultTitle
	return launcherview.AboutSettingsView(launcherview.AboutSettingsProps{
		Width: width, Height: height, AppIcon: a.imageFor(appIconImageSource), Version: version,
		Description: a.translate("i18n:ui_about_description"), Status: status, Theme: theme,
		Links: []launcherview.AboutLink{
			{ID: "about-open-onboarding-button", Label: a.translate("i18n:ui_about_onboarding"), Icon: a.imageForTint(settingControlIconSource("onboarding"), &iconTint, physicalImageSize(18, imageScale)), OnTap: a.openAboutOnboarding},
			{ID: "about-link-documentation", Label: a.translate("i18n:ui_about_docs"), Icon: a.imageForTint(settingControlIconSource("document"), &iconTint, physicalImageSize(18, imageScale)), OnTap: func() { a.openAboutLink("https://wox-launcher.github.io/Wox/#/") }},
			{ID: "about-link-github", Label: a.translate("i18n:ui_about_github"), Icon: a.imageForTint(settingControlIconSource("code"), &iconTint, physicalImageSize(18, imageScale)), OnTap: func() { a.openAboutLink("https://github.com/Wox-launcher/Wox") }},
		},
	})
}

// openAboutOnboarding uses the same dedicated-window entry point as startup.
func (a *App) openAboutOnboarding() {
	if err := a.OpenOnboarding(context.Background()); err != nil {
		log.Printf("open About onboarding: %v", err)
		a.aboutSettings.SetError(err.Error())
	}
}

// openAboutLink records native browser failures in the existing About status state.
func (a *App) openAboutLink(target string) {
	if err := a.settingsNativeWindow().OpenExternalURL(target); err != nil {
		log.Printf("open About link: %v", err)
		a.aboutSettings.SetError(err.Error())
	}
}
