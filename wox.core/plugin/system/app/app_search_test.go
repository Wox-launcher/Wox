package app

import (
	"context"
	"slices"
	"testing"
	"wox/plugin"
	"wox/util"
)

func TestWindowsSettingsSearchCandidates(t *testing.T) {
	info := appInfo{
		Name:            "i18n:plugin_app_windows_settings_windows_update",
		Path:            "ms-settings:windowsupdate",
		Type:            AppTypeWindowsSetting,
		SearchableNames: []string{"software updates"},
	}

	want := []string{"Windows Update", "software updates"}
	if got := info.GetSearchCandidates("Windows Update"); !slices.Equal(got, want) {
		t.Fatalf("search candidates = %q, want %q", got, want)
	}
}

func TestUWPSearchCandidates(t *testing.T) {
	info := appInfo{
		Name:            "天气",
		Path:            `shell:AppsFolder\Microsoft.BingWeather_8wekyb3d8bbwe!App`,
		Type:            AppTypeUWP,
		SearchableNames: []string{"Weather"},
	}

	want := []string{"天气", "Weather"}
	if got := info.GetSearchCandidates(info.Name); !slices.Equal(got, want) {
		t.Fatalf("search candidates = %q, want %q", got, want)
	}
}

type trackedRefreshTestAPI struct {
	emptyAPIImpl
	requested []string
}

func (api *trackedRefreshTestAPI) IsVisible(context.Context) bool {
	return true
}

func (api *trackedRefreshTestAPI) GetUpdatableResult(ctx context.Context, resultID string) *plugin.UpdatableResult {
	api.requested = append(api.requested, util.GetContextQueryId(ctx)+":"+resultID)
	return nil
}

func TestTrackedResultsSkipsStaleQueriesOnRefresh(t *testing.T) {
	api := &trackedRefreshTestAPI{}
	p := &ApplicationPlugin{api: api}
	p.trackResult("session-1", "query-1", "result-old", appInfo{Name: "Old"})
	p.trackResult("session-1", "query-2", "result-new", appInfo{Name: "New"})

	// Simulate an unpurged stale entry in trackedResults alongside the latest query.
	p.trackedResults.Store("result-stale", trackedAppResult{
		sessionID: "session-1",
		queryID:   "query-1",
		info:      appInfo{Name: "Stale"},
	})

	p.refreshRunningApps(context.Background())

	if len(api.requested) != 1 || api.requested[0] != "query-2:result-new" {
		t.Fatalf("GetUpdatableResult calls = %v, want only [query-2:result-new]", api.requested)
	}
	if _, ok := p.trackedResults.Load("result-old"); ok {
		t.Fatal("advanceTrackedQuery left result-old in trackedResults")
	}
	if _, ok := p.trackedResults.Load("result-stale"); ok {
		t.Fatal("refreshRunningApps did not prune stale query entry")
	}
}
