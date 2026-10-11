//go:build wox_automation

package launcher

import (
	"fmt"
	"strings"

	"wox/ui/automation"

	woxui "wox/ui/runtime"
)

const automationPerfCatalogCount = 500

// AutomationQueryState rejects stale retained rows while observing result readiness.
func (a *App) AutomationQueryState() (automation.QueryState, error) {
	target := a.resolveAutomationTarget()
	var state automation.QueryState
	err := target.runOnUI("read automation query state", func() {
		state.QueryID = target.query.QueryID
		state.QueryText = target.query.QueryText
		state.ResultCount = len(target.results)
		if target.host == nil || target.resultsQueryID != state.QueryID {
			return
		}
		snapshot := target.host.Snapshot()
		inputMatches, frameComplete, hasRows := false, false, false
		currentIDs := make(map[string]bool, len(target.results))
		for _, result := range target.results {
			currentIDs["launcher.result."+result.ID] = true
		}
		for _, node := range snapshot.Tree.Nodes {
			switch node.AutomationID {
			case "launcher.query.input":
				inputMatches = node.Value == state.QueryText
			case "launcher.results":
				frameComplete = node.Value == "complete"
			default:
				if strings.HasPrefix(node.AutomationID, "launcher.result.") {
					hasRows = true
					state.Visible = state.Visible || currentIDs[node.AutomationID]
				}
			}
		}
		state.Visible = state.Visible && inputMatches
		state.Complete = inputMatches && target.queryComplete && frameComplete && (state.Visible || (len(target.results) == 0 && !hasRows))
	})
	return state, err
}

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
