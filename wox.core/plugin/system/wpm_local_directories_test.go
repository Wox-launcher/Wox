package system

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
	"wox/plugin"
	"wox/util"
)

type wpmLocalDirectoriesTestAPI struct {
	plugin.API

	mu       sync.Mutex
	settings map[string]string
}

func (a *wpmLocalDirectoriesTestAPI) GetSetting(ctx context.Context, key string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings[key]
}

func (a *wpmLocalDirectoriesTestAPI) SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool) {
	a.mu.Lock()
	a.settings[key] = value
	a.mu.Unlock()
}

func (a *wpmLocalDirectoriesTestAPI) Log(ctx context.Context, level plugin.LogLevel, msg string) {
}

func TestLocalPluginJSONUsesTableColumnKey(t *testing.T) {
	data, err := json.Marshal(LocalPlugin{Path: `D:\dev\plugin`})
	if err != nil {
		t.Fatal(err)
	}
	raw := string(data)
	if !strings.Contains(raw, `"path":`) {
		t.Fatalf("marshaled local plugin = %s, want path key", raw)
	}
	if strings.Contains(raw, `"Path":`) {
		t.Fatalf("marshaled local plugin = %s, must not use Path", raw)
	}

	var rows []LocalPlugin
	if err := json.Unmarshal([]byte(`[{"Path":"D:\\legacy"},{"path":"D:\\current"}]`), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Path != `D:\legacy` || rows[1].Path != `D:\current` {
		t.Fatalf("unmarshaled rows = %#v", rows)
	}
}

func TestApplyLocalPluginDirectoriesRemovesMissingRows(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	api := &wpmLocalDirectoriesTestAPI{settings: map[string]string{}}
	w := &WPMPlugin{
		api:                    api,
		reloadPluginTimers:     util.NewHashMap[string, *time.Timer](),
		localPluginDirectories: []string{dirA, dirB},
		localPlugins: []localPlugin{
			{metadata: plugin.Metadata{Id: "plugin-a", Directory: dirA}},
			{metadata: plugin.Metadata{Id: "plugin-b", Directory: dirB}},
		},
	}

	payload, err := json.Marshal([]LocalPlugin{{Path: dirA}})
	if err != nil {
		t.Fatal(err)
	}
	w.applyLocalPluginDirectories(context.Background(), string(payload))

	if len(w.localPluginDirectories) != 1 || !sameLocalPluginDirectory(w.localPluginDirectories[0], dirA) {
		t.Fatalf("directories = %#v, want only %s", w.localPluginDirectories, dirA)
	}
	if len(w.localPlugins) != 1 || w.localPlugins[0].metadata.Id != "plugin-a" {
		t.Fatalf("local plugins = %#v, want only plugin-a", w.localPlugins)
	}
}

func TestSaveLocalPluginDirectoriesWritesPathKey(t *testing.T) {
	dir := t.TempDir()
	api := &wpmLocalDirectoriesTestAPI{settings: map[string]string{}}
	w := &WPMPlugin{
		api:                    api,
		localPluginDirectories: []string{dir},
		reloadPluginTimers:     util.NewHashMap[string, *time.Timer](),
	}
	w.saveLocalPluginDirectories(context.Background())

	raw := api.GetSetting(context.Background(), localPluginDirectoriesKey)
	if !strings.Contains(raw, `"path":`) || strings.Contains(raw, `"Path":`) {
		t.Fatalf("saved setting = %s, want path key", raw)
	}
	var rows []LocalPlugin
	if err := json.Unmarshal([]byte(raw), &rows); err != nil || len(rows) != 1 || !sameLocalPluginDirectory(rows[0].Path, dir) {
		t.Fatalf("saved setting = %s", raw)
	}
}

func TestParseLocalPluginDirectorySettingsAcceptsLegacyPath(t *testing.T) {
	rows, err := parseLocalPluginDirectorySettings(`[{"Path":"D:\\legacy"}]`)
	if err != nil || len(rows) != 1 || rows[0].Path != `D:\legacy` {
		t.Fatalf("parsed = %#v err=%v", rows, err)
	}
}

func TestRemoveLocalPluginDirectoryUpdatesList(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	api := &wpmLocalDirectoriesTestAPI{settings: map[string]string{}}
	w := &WPMPlugin{
		api:                    api,
		localPluginDirectories: []string{dirA, dirB},
		localPlugins: []localPlugin{
			{metadata: plugin.Metadata{Id: "plugin-a", Directory: dirA}},
			{metadata: plugin.Metadata{Id: "plugin-b", Directory: dirB}},
		},
		reloadPluginTimers: util.NewHashMap[string, *time.Timer](),
	}

	w.removeLocalPluginDirectory(context.Background(), dirB)
	if len(w.localPlugins) != 1 || w.localPlugins[0].metadata.Id != "plugin-a" {
		t.Fatalf("local plugins = %#v, want only plugin-a", w.localPlugins)
	}
	if containsLocalPluginDirectory(w.localPluginDirectories, dirB) {
		t.Fatalf("directories still contain removed path: %#v", w.localPluginDirectories)
	}
}
