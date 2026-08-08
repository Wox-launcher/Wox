package launcher

import (
	"context"
	"time"

	"wox/ui/contract"
)

// usageSettingsSnapshot is the immutable Usage tab state consumed by the view layer.
type usageSettingsSnapshot struct {
	Stats    usageStatsData
	Period   string
	Loading  bool
	Loaded   bool
	Error    string
	Revision uint64
}

// usageSettingsController owns the Usage tab state (stats, period, loading, error).
type usageSettingsController struct {
	deps     CommonDeps
	stats    usageStatsData
	period   string
	loading  bool
	loaded   bool
	errMsg   string
	revision uint64
}

func newUsageSettingsController(deps CommonDeps) *usageSettingsController {
	return &usageSettingsController{deps: deps, period: "30d"}
}

// CurrentPeriod returns the active reporting period, defaulting to "30d".
func (c *usageSettingsController) CurrentPeriod() string {
	if c.period == "" {
		return "30d"
	}
	return c.period
}

// Reload fetches one report period; ignores responses superseded by a later selection.
func (c *usageSettingsController) Reload(ctx context.Context, service contract.UsageSettingsServices, sessionID string, period string) {
	period = normalizeUsagePeriod(period)
	revision := uint64(0)
	if !c.deps.OnUI("start usage settings reload", func() {
		c.revision++
		revision = c.revision
		c.period = period
		c.loading = true
		c.errMsg = ""
		c.deps.Invalidate()
	}) {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	loaded, err := service.UsageStats(timeoutCtx, sessionID, period)
	data := usageStatsFromContract(loaded)

	c.deps.OnUI("finish usage settings reload", func() {
		if revision != c.revision {
			return
		}
		c.loading = false
		if err != nil {
			c.errMsg = err.Error()
		} else {
			data.Period = normalizeUsagePeriod(data.Period)
			c.stats = data
			c.period = data.Period
			c.loaded = true
		}
		c.deps.Invalidate()
	})
}

// usageStatsFromContract isolates controller report state from core-owned slices.
func usageStatsFromContract(source contract.UsageStats) usageStatsData {
	result := usageStatsData{
		Period:          source.Period,
		PeriodOpened:    source.PeriodOpened,
		PeriodAppLaunch: source.PeriodAppLaunch,
		PeriodAppsUsed:  source.PeriodAppsUsed,
		PeriodActions:   source.PeriodActions,
		UsageDays:       source.UsageDays,
		MostActiveHour:  source.MostActiveHour,
		MostActiveDay:   source.MostActiveDay,
		OpenedByDay:     make([]usageStatsDay, len(source.OpenedByDay)),
		TopApps:         make([]usageStatsItem, len(source.TopApps)),
		TopPlugins:      make([]usageStatsItem, len(source.TopPlugins)),
	}
	for index, day := range source.OpenedByDay {
		result.OpenedByDay[index] = usageStatsDay{Date: day.Date, Count: day.Count}
	}
	for index, item := range source.TopApps {
		result.TopApps[index] = usageStatsItem{ID: item.ID, Name: item.Name, Count: item.Count, Icon: woxImage{ImageType: item.Icon.ImageType, ImageData: item.Icon.ImageData}}
	}
	for index, item := range source.TopPlugins {
		result.TopPlugins[index] = usageStatsItem{ID: item.ID, Name: item.Name, Count: item.Count, Icon: woxImage{ImageType: item.Icon.ImageType, ImageData: item.Icon.ImageData}}
	}
	return result
}

// SetShareError records a share-to-X failure message.
func (c *usageSettingsController) SetShareError(msg string) {
	c.errMsg = msg
	c.deps.Invalidate()
}

// Snapshot returns a copy of the Usage state for the view layer.
func (c *usageSettingsController) Snapshot() usageSettingsSnapshot {
	return usageSettingsSnapshot{
		Stats:    cloneUsageStats(c.stats),
		Period:   c.period,
		Loading:  c.loading,
		Loaded:   c.loaded,
		Error:    c.errMsg,
		Revision: c.revision,
	}
}
