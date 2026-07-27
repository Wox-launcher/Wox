package launcher

import (
	"context"
	"strings"
)

type updateChannelVersion struct {
	Channel       string
	LatestVersion string
	Error         string
}

// updateChannelVersionTrailers formats manifest versions for compact display in the channel picker.
func updateChannelVersionTrailers(versions []updateChannelVersion) map[string]string {
	trailers := make(map[string]string, len(versions))
	for _, version := range versions {
		channel := strings.ToLower(strings.TrimSpace(version.Channel))
		latestVersion := strings.TrimSpace(version.LatestVersion)
		if channel == "" || latestVersion == "" {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(latestVersion), "v") {
			latestVersion = "v" + latestVersion
		}
		trailers[channel] = latestVersion
	}
	return trailers
}

// reloadUpdateChannelVersions keeps the update channel picker backed by the same manifest metadata as Flutter.
// The network reload and the len/loading guard now live in updateSettingsController; this wrapper supplies the
// active ReleaseChannel choice picker callback so the controller stays free of any *App back-dependency.
func (a *App) reloadUpdateChannelVersions() {
	a.updateSettings.Reload(context.Background(), a.client, a.applyUpdateChannelTrailers)
}

// applyUpdateChannelTrailers updates the active ReleaseChannel choice picker with the latest channel versions.
// The picker is owned by the general settings domain; updating it here keeps this cross-domain write
// out of updateSettingsController.
func (a *App) applyUpdateChannelTrailers(versions []updateChannelVersion) {
	a.mu.Lock()
	defer a.mu.Unlock()
	picker := a.generalSettings.ChoicePicker()
	if picker != nil && picker.item.key == "ReleaseChannel" {
		picker.item.trailers = updateChannelVersionTrailers(versions)
	}
}
