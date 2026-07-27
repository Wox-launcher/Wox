package launcher

import (
	"context"
	"log"
	"sync"
	"time"
)

// updateSettingsSnapshot is the immutable Update tab state consumed by the view layer.
type updateSettingsSnapshot struct {
	ChannelVersions []updateChannelVersion
	ChannelsLoading bool
}

// updateSettingsController owns the Update tab state (channel versions and loading flag).
// It mirrors the pre-migration reloadUpdateChannelVersions behavior: the manifest fetch is
// guarded so it runs at most once per session, and failures are logged only (no error field).
type updateSettingsController struct {
	deps            CommonDeps
	mu              sync.RWMutex
	channelVersions []updateChannelVersion
	channelsLoading bool
}

func newUpdateSettingsController(deps CommonDeps) *updateSettingsController {
	return &updateSettingsController{deps: deps}
}

// Reload fetches update channel versions from the backend.
//
// applyTrailers is invoked on success so the caller can update any active ReleaseChannel
// choice picker. The picker is owned by the general settings domain (sharedEditState on App),
// so the controller stays free of any *App back-dependency by delegating that cross-domain
// write to the caller.
//
// The reload is a no-op when a reload is already in flight or when versions have already been
// loaded. This guard preserves the old App-level behavior where the updates tab only fetched
// once per settings window session.
func (c *updateSettingsController) Reload(ctx context.Context, client backendClient, applyTrailers func([]updateChannelVersion)) {
	c.mu.Lock()
	if c.channelsLoading || len(c.channelVersions) > 0 {
		c.mu.Unlock()
		return
	}
	c.channelsLoading = true
	c.mu.Unlock()
	c.deps.Invalidate()

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var versions []updateChannelVersion
	err := client.Post(timeoutCtx, "/updater/channel/versions", map[string]any{}, &versions)

	c.mu.Lock()
	c.channelsLoading = false
	if err == nil {
		c.channelVersions = versions
	}
	c.mu.Unlock()
	if err == nil && applyTrailers != nil {
		applyTrailers(versions)
	}
	if err != nil {
		// Preserve existing behavior: failures are logged only, no error field is stored.
		log.Printf("load update channel versions: %v", err)
	}
	c.deps.Invalidate()
}

// Snapshot returns a copy of the Update state for the view layer.
func (c *updateSettingsController) Snapshot() updateSettingsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return updateSettingsSnapshot{
		ChannelVersions: append([]updateChannelVersion(nil), c.channelVersions...),
		ChannelsLoading: c.channelsLoading,
	}
}
