package launcher

import (
	"context"
	"sync"
	"time"
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
	mu       sync.RWMutex
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
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.period == "" {
		return "30d"
	}
	return c.period
}

// Reload fetches one report period; ignores responses superseded by a later selection.
func (c *usageSettingsController) Reload(ctx context.Context, client backendClient, period string) {
	period = normalizeUsagePeriod(period)
	c.mu.Lock()
	c.revision++
	revision := c.revision
	c.period = period
	c.loading = true
	c.errMsg = ""
	c.mu.Unlock()
	c.deps.Invalidate()

	timeoutCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var data usageStatsData
	err := client.Post(timeoutCtx, "/usage/stats", map[string]string{"Period": period}, &data)

	c.mu.Lock()
	if revision != c.revision {
		c.mu.Unlock()
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
	c.mu.Unlock()
	c.deps.Invalidate()
}

// SetShareError records a share-to-X failure message.
func (c *usageSettingsController) SetShareError(msg string) {
	c.mu.Lock()
	c.errMsg = msg
	c.mu.Unlock()
	c.deps.Invalidate()
}

// Snapshot returns a copy of the Usage state for the view layer.
func (c *usageSettingsController) Snapshot() usageSettingsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return usageSettingsSnapshot{
		Stats:    cloneUsageStats(c.stats),
		Period:   c.period,
		Loading:  c.loading,
		Loaded:   c.loaded,
		Error:    c.errMsg,
		Revision: c.revision,
	}
}
