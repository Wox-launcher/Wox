package launcher

import (
	"context"
	"log"
	"time"

	"wox/ui/contract"
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
func (c *updateSettingsController) Reload(ctx context.Context, service contract.UpdateSettingsServices, sessionID string, applyTrailers func([]updateChannelVersion)) {
	shouldLoad := false
	if !c.deps.OnUI("start update settings reload", func() {
		if c.channelsLoading || len(c.channelVersions) > 0 {
			return
		}
		c.channelsLoading = true
		shouldLoad = true
		c.deps.Invalidate()
	}) || !shouldLoad {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	loaded, err := service.UpdateChannelVersions(timeoutCtx, sessionID)
	versions := make([]updateChannelVersion, len(loaded))
	for index, version := range loaded {
		versions[index] = updateChannelVersion{
			Channel:       version.Channel,
			LatestVersion: version.LatestVersion,
			Error:         version.Error,
		}
	}

	c.deps.OnUI("finish update settings reload", func() {
		c.channelsLoading = false
		if err == nil {
			c.channelVersions = versions
			if applyTrailers != nil {
				applyTrailers(versions)
			}
		}
		c.deps.Invalidate()
	})
	if err != nil {
		// Preserve existing behavior: failures are logged only, no error field is stored.
		log.Printf("load update channel versions: %v", err)
	}
}

// Snapshot returns a copy of the Update state for the view layer.
func (c *updateSettingsController) Snapshot() updateSettingsSnapshot {
	return updateSettingsSnapshot{
		ChannelVersions: append([]updateChannelVersion(nil), c.channelVersions...),
		ChannelsLoading: c.channelsLoading,
	}
}
