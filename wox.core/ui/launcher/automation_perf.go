//go:build wox_automation

package launcher

import (
	"fmt"

	woxui "wox/ui/runtime"
)

const automationPerfCatalogCount = 500

// InstallAutomationPerfFixture installs deterministic large catalogs used by performance smoke.
func (a *App) InstallAutomationPerfFixture(name string) error {
	if a == nil {
		return fmt.Errorf("launcher app is not initialized")
	}
	var installErr error
	if err := woxui.Call(func() {
		switch name {
		case "catalog-500":
			a.installCatalogPerfFixture(automationPerfCatalogCount)
		default:
			installErr = fmt.Errorf("unknown perf fixture %q", name)
		}
	}); err != nil {
		return err
	}
	return installErr
}

func (a *App) installCatalogPerfFixture(count int) {
	plugins := make([]pluginSettingsPlugin, 0, count)
	for index := range count {
		plugins = append(plugins, pluginSettingsPlugin{
			ID:          fmt.Sprintf("perf-plugin-%04d", index),
			Name:        fmt.Sprintf("Perf Plugin %04d", index),
			Description: "Deterministic catalog fixture",
			Author:      "Wox",
			Version:     "1.0.0",
			Runtime:     "Go",
			Icon:        appIconImageSource,
			IsInstalled: true,
		})
	}
	a.pluginSettings.SetPlugins(plugins)
	a.pluginSettings.cachePlugins(false, plugins)
	a.pluginSettings.SetPluginsStore(false)
	a.pluginSettings.SetPluginsLoaded(true)
	a.pluginSettings.SetPluginsLoading(false)
	a.pluginSettings.SetPluginsError("")
	if count > 0 {
		a.pluginSettings.SetSelected(0)
	}

	themes := make([]themeSettingsTheme, 0, count)
	for index := range count {
		themes = append(themes, themeSettingsTheme{
			ID:          fmt.Sprintf("perf-theme-%04d", index),
			Name:        fmt.Sprintf("Perf Theme %04d", index),
			Author:      "Wox",
			Version:     "1.0.0",
			Description: "Deterministic catalog fixture",
			IsInstalled: true,
		})
	}
	a.themeSettings.SetThemesMode("installed")
	a.themeSettings.SetThemes(themes)
	a.themeSettings.SetThemesLoaded(true)
	a.themeSettings.SetThemesLoading(false)
	a.themeSettings.SetThemesError("")
	if count > 0 {
		a.themeSettings.SetThemeSelected(0)
	}
	a.invalidateSettingsWindow()
}

func (a *App) resetAutomationPerfCatalog() {
	if a.pluginSettings != nil {
		a.pluginSettings.SetPlugins(nil)
		a.pluginSettings.cachePlugins(false, nil)
		a.pluginSettings.cachePlugins(true, nil)
		a.pluginSettings.invalidateCachedPlugins(false)
		a.pluginSettings.invalidateCachedPlugins(true)
		a.pluginSettings.SetPluginsLoaded(false)
		a.pluginSettings.SetPluginsLoading(false)
		a.pluginSettings.SetPluginsError("")
		a.pluginSettings.SetSelected(-1)
	}
	if a.themeSettings != nil {
		a.themeSettings.SetThemes(nil)
		a.themeSettings.SetThemesLoaded(false)
		a.themeSettings.SetThemesLoading(false)
		a.themeSettings.SetThemesError("")
		a.themeSettings.SetThemeSelected(-1)
	}
}
