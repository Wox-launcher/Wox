package launcher

import (
	"context"
	"log"
	"strings"
	"time"
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
func (a *App) reloadUpdateChannelVersions() {
	a.mu.Lock()
	if a.updateChannelsLoading || len(a.updateChannelVersions) > 0 {
		a.mu.Unlock()
		return
	}
	a.updateChannelsLoading = true
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var versions []updateChannelVersion
	err := a.client.Post(ctx, "/updater/channel/versions", map[string]any{}, &versions)

	a.mu.Lock()
	a.updateChannelsLoading = false
	if err == nil {
		a.updateChannelVersions = versions
		if a.settingChoicePicker != nil && a.settingChoicePicker.item.key == "ReleaseChannel" {
			a.settingChoicePicker.item.trailers = updateChannelVersionTrailers(versions)
		}
	}
	a.mu.Unlock()
	if err != nil {
		log.Printf("load update channel versions: %v", err)
	}
	a.invalidateSettingsWindow()
}
