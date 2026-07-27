package launcher

import (
	"context"
	"sync"
	"time"

	"wox/ui/contract"
)

// aboutSettingsSnapshot is the immutable About tab state consumed by the view layer.
type aboutSettingsSnapshot struct {
	Version string
	Loading bool
	Loaded  bool
	Error   string
}

// aboutSettingsController owns the About tab state (version, loading, error).
type aboutSettingsController struct {
	deps    CommonDeps
	mu      sync.RWMutex
	version string
	loading bool
	loaded  bool
	errMsg  string
}

func newAboutSettingsController(deps CommonDeps) *aboutSettingsController {
	return &aboutSettingsController{deps: deps}
}

// Reload fetches the running core version. Mirrors the old reloadAboutVersion behavior:
// it is a no-op if a reload is already in flight, clears any prior error, and invalidates
// the view before and after the service call.
func (c *aboutSettingsController) Reload(ctx context.Context, service contract.AboutSettingsServices, sessionID string) {
	c.mu.Lock()
	if c.loading {
		c.mu.Unlock()
		return
	}
	c.loading = true
	c.errMsg = ""
	c.mu.Unlock()
	c.deps.Invalidate()

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	version, err := service.Version(timeoutCtx, sessionID)

	c.mu.Lock()
	c.loading = false
	if err != nil {
		c.errMsg = err.Error()
	} else {
		c.version = version
		c.loaded = true
	}
	c.mu.Unlock()
	c.deps.Invalidate()
}

// SetError records an error from outside the reload path (e.g. onboarding/open link failures).
func (c *aboutSettingsController) SetError(msg string) {
	c.mu.Lock()
	c.errMsg = msg
	c.mu.Unlock()
	c.deps.Invalidate()
}

// Snapshot returns a copy of the About state for the view layer.
func (c *aboutSettingsController) Snapshot() aboutSettingsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return aboutSettingsSnapshot{
		Version: c.version,
		Loading: c.loading,
		Loaded:  c.loaded,
		Error:   c.errMsg,
	}
}
