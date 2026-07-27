package launcher

import (
	"context"
	"sync"
	"time"
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

// backendClient is the narrow interface about (and later controllers) need from a.client.
// a.client is coreclient.Backend whose Post takes `data any`, so we match that signature here.
type backendClient interface {
	Post(ctx context.Context, path string, data any, target any) error
}

func newAboutSettingsController(deps CommonDeps) *aboutSettingsController {
	return &aboutSettingsController{deps: deps}
}

// Reload fetches the running core version. Mirrors the old reloadAboutVersion behavior:
// it is a no-op if a reload is already in flight, clears any prior error, and invalidates
// the view before and after the network call.
func (c *aboutSettingsController) Reload(ctx context.Context, client backendClient) {
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
	var version string
	err := client.Post(timeoutCtx, "/version", map[string]any{}, &version)

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
