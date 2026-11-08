package launcher

import (
	"context"
	"log"

	"wox/resource/licenses"
	launcherview "wox/ui/launcher/view"
	woxwidget "wox/ui/widget"
)

// buildAboutSettingsPage maps core version state and external actions into the About view.
func (a *App) buildAboutSettingsPage(snapshot settingsSnapshot, width, height, imageScale float32) woxwidget.Widget {
	licensesLabel := a.language.translate("i18n:ui_about_licenses")
	if snapshot.about.LicensesOpen {
		return launcherview.AboutLicensesView(launcherview.AboutLicensesProps{
			Width: width, Height: height, Theme: snapshot.palette,
			Window: a.settingsWindow.settingsNativeWindow(),
			Title:  licensesLabel, Back: a.language.translate("i18n:ui_back"),
			Empty:   a.language.translate("i18n:ui_about_licenses_unavailable"),
			Entries: licenses.Entries(), Selected: snapshot.about.SelectedLicense,
			OnBack:   func() { a.aboutSettings.setLicensePage(snapshot.about.SelectedLicense != nil, nil) },
			OnSelect: func(entry licenses.Entry) { a.aboutSettings.setLicensePage(true, &entry) },
		})
	}
	version := snapshot.about.Version
	if version == "" && snapshot.about.Loading {
		version = a.language.translate("i18n:ui_about_version") + "…"
	}
	if version == "" {
		version = a.language.translate("i18n:ui_about_version")
	}
	status := ""
	if snapshot.about.Error != "" {
		status = snapshot.about.Error
	}
	theme := snapshot.palette
	iconTint := theme.Text
	return launcherview.AboutSettingsView(launcherview.AboutSettingsProps{
		Width: width, Height: height, AppIcon: a.imageForSurface(appIconImageSource, 256, settingsPalette().Background), Version: version,
		Description: a.language.translate("i18n:ui_about_description"), Status: status, Theme: theme,
		Links: []launcherview.AboutLink{
			{ID: "about-open-onboarding-button", Label: a.language.translate("i18n:ui_about_onboarding"), Icon: a.imageForTint(settingControlIconSource("onboarding"), &iconTint, physicalImageSize(18, imageScale)), OnTap: a.openAboutOnboarding},
			{ID: "about-link-documentation", Label: a.language.translate("i18n:ui_about_docs"), Icon: a.imageForTint(settingControlIconSource("documentation"), &iconTint, physicalImageSize(18, imageScale)), OnTap: func() { a.openAboutLink("https://www.woxlauncher.com/") }},
			{ID: "about-link-github", Label: a.language.translate("i18n:ui_about_github"), Icon: a.imageForTint(pluginMetadataIconSource("github"), &iconTint, physicalImageSize(18, imageScale)), OnTap: func() { a.openAboutLink("https://github.com/Wox-launcher/Wox") }},
			{ID: "about-open-licenses", Label: licensesLabel, Icon: a.imageForTint(settingControlIconSource("document"), &iconTint, physicalImageSize(18, imageScale)), OnTap: a.aboutSettings.ShowLicenses},
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
	if err := a.settingsWindow.settingsNativeWindow().OpenExternalURL(target); err != nil {
		log.Printf("open About link: %v", err)
		a.aboutSettings.SetError(err.Error())
	}
}
