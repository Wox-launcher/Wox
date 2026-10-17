package app

import (
	"context"
	"os"
	"testing"
)

func TestListControlPanelEntriesIncludesAppletsAndTasks(t *testing.T) {
	entries, err := listControlPanelEntries()
	if err != nil {
		t.Fatalf("listControlPanelEntries: %v", err)
	}
	applets := 0
	tasks := 0
	apps := make([]appInfo, 0, len(entries))
	for _, entry := range entries {
		switch {
		case isControlPanelTaskPath(entry.Path):
			tasks++
		case isControlPanelAppletPath(entry.Path):
			applets++
		default:
			t.Fatalf("unexpected control panel path %q (%s)", entry.Path, entry.Name)
		}
		if entry.Name == "" {
			t.Fatalf("empty name for %s", entry.Path)
		}
		apps = append(apps, appInfo{
			Name:          entry.Name,
			Path:          entry.Path,
			Icon:          appIcon,
			Type:          AppTypeWindowsSetting,
			IsDefaultIcon: true,
		})
	}
	if applets < 10 || tasks < 20 {
		t.Fatalf("applets=%d tasks=%d", applets, tasks)
	}

	(&WindowsRetriever{}).attachControlPanelIcons(context.Background(), apps)
	failed := 0
	var appletIcon string
	var taskIcon string
	for _, app := range apps {
		if app.IsDefaultIcon || app.Icon.ImageData == "" {
			failed++
			continue
		}
		if appletIcon == "" && isControlPanelAppletPath(app.Path) {
			appletIcon = app.Icon.ImageData
		}
		if taskIcon == "" && isControlPanelTaskPath(app.Path) {
			taskIcon = app.Icon.ImageData
		}
	}
	if failed > 0 {
		t.Fatalf("control panel icons failed for %d of %d items", failed, len(apps))
	}
	for _, sample := range []string{appletIcon, taskIcon} {
		info, err := os.Stat(sample)
		if err != nil {
			t.Fatalf("stat icon: %v", err)
		}
		if info.Size() < 50 {
			t.Fatalf("icon %s is only %d bytes", sample, info.Size())
		}
	}
}
