package launcher

import (
	launcherview "wox/ui/launcher/view"
	woxwidget "wox/ui/widget"
)

// buildUsageSettingsPage maps the local analytics snapshot into its portable view.
func (a *App) buildUsageSettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	periods := make([]launcherview.UsagePeriod, 0, 4)
	for _, id := range []string{"7d", "30d", "365d", "all"} {
		id := id
		periods = append(periods, launcherview.UsagePeriod{ID: id, Label: usagePeriodLabel(id), Selected: id == snapshot.usagePeriod, OnSelect: func() { go a.reloadUsageStats(id) }})
	}
	days := make([]launcherview.UsageDay, 0, len(snapshot.usage.OpenedByDay))
	for _, day := range snapshot.usage.OpenedByDay {
		days = append(days, launcherview.UsageDay{Date: day.Date, Count: day.Count})
	}
	return launcherview.UsageSettingsView(launcherview.UsageSettingsProps{
		Width: width, Height: height, Theme: snapshot.palette.componentTheme(), PeriodLabel: usagePeriodLabel(snapshot.usagePeriod), Periods: periods,
		Error: snapshot.usageError, Loading: snapshot.usageLoading && len(days) == 0,
		KPIs: []launcherview.UsageKPI{{Label: "Opened", Value: snapshot.usage.PeriodOpened}, {Label: "App launches", Value: snapshot.usage.PeriodAppLaunch}, {Label: "Apps used", Value: snapshot.usage.PeriodAppsUsed}, {Label: "Actions", Value: snapshot.usage.PeriodActions}},
		Days: days, TopApps: usageRankingItems(snapshot.usage.TopApps), TopPlugins: usageRankingItems(snapshot.usage.TopPlugins),
	})
}

// usageRankingItems normalizes and limits ranking rows before rendering.
func usageRankingItems(items []usageStatsItem) []launcherview.UsageRankingItem {
	result := make([]launcherview.UsageRankingItem, 0, min(5, len(items)))
	for index, item := range items {
		if index == 5 {
			break
		}
		name := item.Name
		if name == "" {
			name = item.ID
		}
		result = append(result, launcherview.UsageRankingItem{Name: name, Count: item.Count})
	}
	return result
}
