package launcher

import (
	"fmt"
	"strings"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

// buildUsageSettingsPage maps the local analytics snapshot into its portable view.
func (a *App) buildUsageSettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	theme := snapshot.palette.componentTheme()
	periods := make([]launcherview.UsagePeriod, 0, 4)
	for _, id := range []string{"7d", "30d", "365d", "all"} {
		periods = append(periods, launcherview.UsagePeriod{
			ID: id, Label: a.translate("i18n:" + usagePeriodLabelKey(id)), Selected: id == snapshot.usage.Period,
			OnSelect: func() {
				util.Go(a.lifecycleCtx, "reload usage stats", func() {
					a.reloadUsageStats(id)
				})
			},
		})
	}
	days := make([]launcherview.UsageDay, 0, len(snapshot.usage.Stats.OpenedByDay))
	for _, day := range snapshot.usage.Stats.OpenedByDay {
		days = append(days, launcherview.UsageDay{Date: day.Date, Count: day.Count})
	}
	monthLabels := make([]string, 12)
	for month := 1; month <= 12; month++ {
		monthLabels[month-1] = a.translate(fmt.Sprintf("i18n:ui_month_short_%d", month))
	}
	periodLabel := a.translate("i18n:" + usagePeriodLabelKey(snapshot.usage.Period))
	overview := strings.ReplaceAll(a.translate("i18n:ui_usage_overview"), "{period}", periodLabel)

	blueAccent := woxui.Color{R: 59, G: 130, B: 246, A: 255}
	tealAccent := woxui.Color{R: 20, G: 184, B: 166, A: 255}
	amberAccent := woxui.Color{R: 245, G: 158, B: 11, A: 255}
	violetAccent := woxui.Color{R: 139, G: 92, B: 246, A: 255}
	greenAccent := woxui.Color{R: 34, G: 197, B: 94, A: 255}
	silverAccent := woxui.Color{R: 163, G: 169, B: 183, A: 255}
	bronzeAccent := woxui.Color{R: 190, G: 121, B: 69, A: 255}

	return launcherview.UsageSettingsView(launcherview.UsageSettingsProps{
		Width: width, Height: height, Theme: theme, Title: a.translate("i18n:ui_usage"), Overview: overview,
		ShareLabel: a.translate("i18n:ui_usage_share_x"), Periods: periods, Error: snapshot.usage.Error, Loading: snapshot.usage.Loading,
		ActivityTitle: a.translate("i18n:ui_usage_opened_by_day"), TopAppsTitle: a.translate("i18n:ui_usage_top_apps"),
		TopPluginsTitle: a.translate("i18n:ui_usage_top_plugins"), EmptyLabel: a.translate("i18n:ui_usage_no_data"),
		MonthLabels: monthLabels, OnShare: a.shareUsageToX,
		KPIs: []launcherview.UsageKPI{
			{Label: a.translate("i18n:ui_usage_opened"), Value: snapshot.usage.Stats.PeriodOpened, Icon: a.imageForTint(usageIconSource("visibility"), &blueAccent, 22), Accent: blueAccent},
			{Label: a.translate("i18n:ui_usage_app_launches"), Value: snapshot.usage.Stats.PeriodAppLaunch, Icon: a.imageForTint(usageIconSource("rocket"), &tealAccent, 22), Accent: tealAccent},
			{Label: a.translate("i18n:ui_usage_apps_used"), Value: snapshot.usage.Stats.PeriodAppsUsed, Icon: a.imageForTint(usageIconSource("apps"), &amberAccent, 22), Accent: amberAccent},
			{Label: a.translate("i18n:ui_usage_actions"), Value: snapshot.usage.Stats.PeriodActions, Icon: a.imageForTint(usageIconSource("bolt"), &violetAccent, 22), Accent: violetAccent},
		},
		Days: days, HeatmapAccent: greenAccent,
		TopApps: usageRankingItems(a, snapshot.usage.Stats.TopApps, true), TopPlugins: usageRankingItems(a, snapshot.usage.Stats.TopPlugins, false),
		ShareIcon:       a.imageForTint(usageIconSource("share"), &theme.ResultTitle, 16),
		CalendarIcon:    a.imageForTint(usageIconSource("calendar"), &theme.ResultTitle, 16),
		AppsIcon:        a.imageForTint(usageIconSource("apps"), &theme.ResultTitle, 16),
		PluginsIcon:     a.imageForTint(usageIconSource("extension"), &theme.ResultTitle, 16),
		AppFallbackIcon: a.imageForTint(usageIconSource("apps"), &blueAccent, 14),
		RankIcons: []*woxui.Image{
			a.imageForTint(usageIconSource("trophy"), &amberAccent, 16),
			a.imageForTint(usageIconSource("medal"), &silverAccent, 16),
			a.imageForTint(usageIconSource("medal"), &bronzeAccent, 16),
		},
		AppAccent: blueAccent, PluginAccent: violetAccent,
	})
}

// usageRankingItems normalizes and limits ranking rows before rendering.
func usageRankingItems(a *App, items []usageStatsItem, includeIcons bool) []launcherview.UsageRankingItem {
	result := make([]launcherview.UsageRankingItem, 0, min(10, len(items)))
	for index, item := range items {
		if index == 10 {
			break
		}
		name := item.Name
		if name == "" {
			name = item.ID
		}
		var icon *woxui.Image
		if includeIcons {
			icon = a.imageFor(item.Icon)
		}
		result = append(result, launcherview.UsageRankingItem{Name: name, Count: item.Count, Icon: icon})
	}
	return result
}
