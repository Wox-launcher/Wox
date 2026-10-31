package plugin

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"wox/database"
	"wox/setting"
	"wox/util"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/require"
)

type migrationTestHost struct {
	fakeHost
	runtime Runtime
	failNew bool
}

func (h *migrationTestHost) GetRuntime(context.Context) Runtime { return h.runtime }

func (h *migrationTestHost) LoadPlugin(_ context.Context, metadata Metadata, _ string) (Plugin, error) {
	p := &fakeLifecyclePlugin{}
	if h.failNew && metadata.Version == "2.0.0" {
		p.initErr = errors.New("new plugin init failed")
	}
	return p, nil
}

// TestArtifactMigration exercises every cross-format path and rollback after download, identity, and init failures.
func TestArtifactMigration(t *testing.T) {
	for _, oldKind := range []PluginArtifactKind{PluginArtifactScript, PluginArtifactSingleFile, PluginArtifactPackage} {
		for _, newKind := range []PluginArtifactKind{PluginArtifactScript, PluginArtifactSingleFile, PluginArtifactPackage} {
			if oldKind == newKind {
				continue
			}
			t.Run(string(oldKind)+"_to_"+string(newKind), func(t *testing.T) {
				testArtifactMigration(t, oldKind, newKind, "")
			})
		}
	}
	for _, failure := range []string{"download", "identity", "init", "canceled", "collision"} {
		t.Run("script_to_package_"+failure, func(t *testing.T) {
			testArtifactMigration(t, PluginArtifactScript, PluginArtifactPackage, failure)
		})
	}
}

func testArtifactMigration(t *testing.T, oldKind, newKind PluginArtifactKind, failure string) {
	t.Helper()
	initPluginManagerLoadTest(t)
	previousManager := GetPluginManager()
	previousHosts := AllHosts
	managerInstance = newManager()
	t.Cleanup(func() { managerInstance = previousManager; AllHosts = previousHosts })
	scriptHost := &migrationTestHost{runtime: PLUGIN_RUNTIME_SCRIPT, fakeHost: fakeHost{started: true}, failNew: failure == "init"}
	pythonHost := &migrationTestHost{runtime: PLUGIN_RUNTIME_PYTHON, fakeHost: fakeHost{started: true}, failNew: failure == "init"}
	nodeHost := &migrationTestHost{runtime: PLUGIN_RUNTIME_NODEJS, fakeHost: fakeHost{started: true}, failNew: failure == "init"}
	AllHosts = []Host{scriptHost, pythonHost, nodeHost}

	const id = "artifact-migration-test"
	old := Metadata{Id: id, Name: "Migration", Version: "1.0.0", Runtime: string(PLUGIN_RUNTIME_PYTHON), TriggerKeywords: []string{"migration"}}
	var oldPath string
	switch oldKind {
	case PluginArtifactScript:
		old.Runtime = string(PLUGIN_RUNTIME_SCRIPT)
		old.Directory = util.GetLocation().GetUserScriptPluginsDirectory()
		old.Entry = "old.py"
		oldPath = filepath.Join(old.Directory, old.Entry)
	case PluginArtifactSingleFile:
		old.Directory = util.GetLocation().GetUserSingleFilePluginsDirectory()
		old.Entry = "old.py"
		oldPath = filepath.Join(old.Directory, old.Entry)
	case PluginArtifactPackage:
		old.Directory = filepath.Join(util.GetLocation().GetPluginDirectory(), id+"@1.0.0")
		oldPath = old.Directory
	}
	if oldKind == PluginArtifactPackage {
		require.NoError(t, os.MkdirAll(oldPath, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(oldPath, "old.txt"), []byte("old"), 0644))
	} else {
		require.NoError(t, os.MkdirAll(filepath.Dir(oldPath), 0755))
		require.NoError(t, os.WriteFile(oldPath, []byte("old"), 0644))
	}
	host := pythonHost
	if oldKind == PluginArtifactScript {
		host = scriptHost
	}
	ctx := context.Background()
	require.NoError(t, managerInstance.lifecycle.loadHostPlugin(ctx, host, old))
	oldInstance := managerInstance.GetPluginInstanceById(id)
	require.NoError(t, oldInstance.WaitInit(ctx))
	settings := setting.NewPluginSettingStore(database.GetDB(), id)
	require.NoError(t, settings.Set("migration_test", "kept"))
	cacheDir := NewAPI(oldInstance).GetCacheFolder(ctx)
	cachePath := filepath.Join(cacheDir, "cache.txt")
	require.NoError(t, os.WriteFile(cachePath, []byte("cached"), 0644))

	newID := id
	if failure == "identity" {
		newID = "different-plugin"
	}
	var body []byte
	name := "new.py"
	runtime := PLUGIN_RUNTIME_PYTHON
	switch newKind {
	case PluginArtifactPackage:
		name = "new.wox"
		runtime = PLUGIN_RUNTIME_NODEJS
		var buffer bytes.Buffer
		writer := zip.NewWriter(&buffer)
		entry, err := writer.Create("plugin.json")
		require.NoError(t, err)
		_, err = fmt.Fprintf(entry, `{"Id":%q,"Name":"Migration","Version":"2.0.0","Runtime":"NODEJS","Entry":"main.js","TriggerKeywords":["migration"],"SupportedOS":["darwin","windows","linux"]}`, newID)
		require.NoError(t, err)
		require.NoError(t, writer.Close())
		body = buffer.Bytes()
	case PluginArtifactScript:
		runtime = PLUGIN_RUNTIME_SCRIPT
		body = []byte(fmt.Sprintf("# {\n# \"Id\": %q,\n# \"Name\": \"Migration\",\n# \"Version\": \"2.0.0\",\n# \"TriggerKeywords\": [\"migration\"]\n# }\n", newID))
	case PluginArtifactSingleFile:
		body = []byte(fmt.Sprintf("# {\n# \"Id\": %q,\n# \"Name\": \"Migration\",\n# \"Version\": \"2.0.0\",\n# \"MinWoxVersion\": %q,\n# \"Runtime\": \"PYTHON\",\n# \"TriggerKeywords\": [\"migration\"]\n# }\nplugin = object()\n", newID, SingleFilePluginMinWoxVersion))
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if failure == "download" {
			http.Error(w, "download failed", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	manifest := StorePluginManifest{Id: id, Name: "Migration", Version: "2.0.0", MinWoxVersion: SingleFilePluginMinWoxVersion, Runtime: runtime, DownloadUrl: server.URL + "/" + name}
	downgrade := manifest
	downgrade.Version = "0.9.0"
	require.Error(t, (&Store{}).InstallLocal(ctx, downgrade))
	require.Equal(t, oldInstance, managerInstance.GetPluginInstanceById(id))
	require.FileExists(t, oldPathForTest(oldPath, oldKind))
	if failure == "collision" {
		target := filepath.Join(util.GetLocation().GetPluginDirectory(), id+"@2.0.0")
		require.NoError(t, os.MkdirAll(target, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(target, "keep.txt"), []byte("keep"), 0644))
	}
	var err error
	if failure == "canceled" {
		cancelCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		err = (&Store{}).installWithProgress(cancelCtx, manifest, func(string) { cancel() }, false)
	} else {
		err = (&Store{}).InstallLocal(ctx, manifest)
	}
	if failure != "" {
		require.Error(t, err)
		require.FileExists(t, oldPathForTest(oldPath, oldKind))
		require.Equal(t, "1.0.0", managerInstance.GetPluginInstanceById(id).Metadata.Version)
		if newKind == PluginArtifactPackage {
			target := filepath.Join(util.GetLocation().GetPluginDirectory(), id+"@2.0.0")
			if failure == "collision" {
				require.FileExists(t, filepath.Join(target, "keep.txt"))
			} else {
				_, statErr := os.Stat(target)
				require.True(t, os.IsNotExist(statErr))
			}
		}
	} else {
		require.NoError(t, err)
		_, statErr := os.Stat(oldPath)
		require.True(t, os.IsNotExist(statErr))
		instance := managerInstance.GetPluginInstanceById(id)
		require.NotNil(t, instance)
		require.Equal(t, newKind, InstalledPluginArtifactKind(instance))
		require.Equal(t, "2.0.0", instance.Metadata.Version)
	}
	var value string
	require.NoError(t, settings.Get("migration_test", &value))
	require.Equal(t, "kept", value)
	require.FileExists(t, cachePath)
}

func oldPathForTest(path string, kind PluginArtifactKind) string {
	if kind == PluginArtifactPackage {
		return filepath.Join(path, "old.txt")
	}
	return path
}

func TestArtifactMigrationSuppressesFileWatchers(t *testing.T) {
	initSingleFileTestLocation(t)
	lifecycle := newPluginLifecycle()
	scriptPath := filepath.Join(util.GetLocation().GetUserScriptPluginsDirectory(), "old.py")
	singlePath := filepath.Join(util.GetLocation().GetUserSingleFilePluginsDirectory(), "new.py")
	lifecycle.instances = []*Instance{{Metadata: Metadata{Id: "old", Runtime: string(PLUGIN_RUNTIME_SCRIPT), Entry: "old.py"}}}
	setArtifactWatchDeadline(&lifecycle, PluginArtifactScript, scriptPath, math.MaxInt64)
	setArtifactWatchDeadline(&lifecycle, PluginArtifactSingleFile, singlePath, math.MaxInt64)
	lifecycle.handleScriptPluginEvent(context.Background(), fsnotify.Event{Name: scriptPath, Op: fsnotify.Remove})
	lifecycle.handleSingleFilePluginEvent(context.Background(), fsnotify.Event{Name: singlePath, Op: fsnotify.Create})
	require.Len(t, lifecycle.pluginInstancesSnapshot(), 1)
	require.Empty(t, lifecycle.singleFileReloadTimers.Keys())
}
