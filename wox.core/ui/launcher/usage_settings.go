package launcher

import (
	"context"
	"time"
)

type usageStatsData struct {
	Period          string
	PeriodOpened    int64
	PeriodAppLaunch int64
	PeriodAppsUsed  int64
	PeriodActions   int64
	UsageDays       int
	MostActiveHour  int
	MostActiveDay   int
	OpenedByDay     []usageStatsDay
	TopApps         []usageStatsItem
	TopPlugins      []usageStatsItem
}

type usageStatsDay struct {
	Date  string
	Count int64
}

type usageStatsItem struct {
	ID    string `json:"Id"`
	Name  string
	Count int64
}

// cloneUsageStats keeps render snapshots independent from asynchronous report refreshes.
func cloneUsageStats(source usageStatsData) usageStatsData {
	result := source
	result.OpenedByDay = append([]usageStatsDay(nil), source.OpenedByDay...)
	result.TopApps = append([]usageStatsItem(nil), source.TopApps...)
	result.TopPlugins = append([]usageStatsItem(nil), source.TopPlugins...)
	return result
}

func (a *App) currentUsagePeriod() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.usagePeriod == "" {
		return "30d"
	}
	return a.usagePeriod
}

// reloadUsageStats refreshes one report period and ignores responses superseded by a later selection.
func (a *App) reloadUsageStats(period string) {
	period = normalizeUsagePeriod(period)
	a.mu.Lock()
	a.usageRevision++
	revision := a.usageRevision
	a.usagePeriod = period
	a.usageLoading = true
	a.usageError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var data usageStatsData
	err := a.client.Post(ctx, "/usage/stats", map[string]string{"Period": period}, &data)

	a.mu.Lock()
	if revision != a.usageRevision {
		a.mu.Unlock()
		return
	}
	a.usageLoading = false
	if err != nil {
		a.usageError = err.Error()
	} else {
		data.Period = normalizeUsagePeriod(data.Period)
		a.usageStats = data
		a.usagePeriod = data.Period
		a.usageLoaded = true
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func normalizeUsagePeriod(period string) string {
	switch period {
	case "7d", "30d", "365d", "all":
		return period
	default:
		return "30d"
	}
}

func usagePeriodLabel(period string) string {
	switch normalizeUsagePeriod(period) {
	case "7d":
		return "7 days"
	case "365d":
		return "365 days"
	case "all":
		return "All time"
	default:
		return "30 days"
	}
}
