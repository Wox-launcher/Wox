package app

import (
	"slices"
	"testing"
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
