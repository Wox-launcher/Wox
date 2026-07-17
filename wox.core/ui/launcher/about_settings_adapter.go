package launcher

import (
	"log"

	launcherview "wox/ui/launcher/view"
	woxwidget "wox/ui/widget"
)

// buildAboutSettingsPage maps core version state and external actions into the About view.
func (a *App) buildAboutSettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	version := snapshot.aboutVersion
	if version == "" && snapshot.aboutLoading {
		version = "Loading version…"
	}
	if version == "" {
		version = "Version unavailable"
	}
	status := ""
	if snapshot.aboutError != "" {
		status = "Could not read version: " + snapshot.aboutError
	}
	return launcherview.AboutSettingsView(launcherview.AboutSettingsProps{
		Width: width, Height: height, Version: version, Status: status, Theme: snapshot.palette.componentTheme(),
		Links: []launcherview.AboutLink{
			{ID: "Documentation", Label: "Documentation", OnTap: func() { a.openAboutLink("https://wox-launcher.github.io/Wox/#/") }},
			{ID: "GitHub", Label: "GitHub", OnTap: func() { a.openAboutLink("https://github.com/Wox-launcher/Wox") }},
		},
	})
}

// openAboutLink records native browser failures in the existing About status state.
func (a *App) openAboutLink(target string) {
	if err := a.settingsNativeWindow().OpenExternalURL(target); err != nil {
		log.Printf("open About link: %v", err)
		a.mu.Lock()
		a.aboutError = err.Error()
		a.mu.Unlock()
		a.invalidateSettingsWindow()
	}
}
