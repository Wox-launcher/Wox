package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"wox/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCacheFolderCreatesPluginDirectory(t *testing.T) {
	initSingleFileTestLocation(t)

	pluginID := "a8c4e2b1-6d3f-4a19-9e7c-2b5f0d8a1c63"
	api := NewAPI(&Instance{Metadata: Metadata{Id: pluginID, Name: "Gif Search"}})
	folder := api.GetCacheFolder(context.Background())

	expected, err := util.GetLocation().GetPluginCacheDirectory(pluginID)
	require.NoError(t, err)
	assert.Equal(t, expected, folder)

	info, statErr := os.Stat(folder)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

func TestUninstallPreservesCacheOnlyWhenReplacingPlugin(t *testing.T) {
	// The process-wide logger keeps Windows file handles open beyond each subtest.
	logRoot := filepath.Join(os.TempDir(), "wox-plugin-tests")
	t.Setenv(util.TestWoxDataDirEnv, logRoot)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(logRoot, "user"))
	require.NoError(t, util.GetLocation().Init())
	GetPluginManager()
	for _, preserveCache := range []bool{true, false} {
		t.Run(map[bool]string{true: "replacement", false: "uninstall"}[preserveCache], func(t *testing.T) {
			initSingleFileTestLocation(t)
			instance := &Instance{Metadata: Metadata{
				Id:        "cached-plugin",
				Runtime:   string(PLUGIN_RUNTIME_PYTHON),
				Directory: util.GetLocation().GetUserSingleFilePluginsDirectory(),
				Entry:     "cached.py",
			}}
			folder := NewAPI(instance).GetCacheFolder(context.Background())
			cacheFile := filepath.Join(folder, "icons.sqlite")
			require.NoError(t, os.WriteFile(cacheFile, []byte("cached icons"), 0644))

			// Keeping settings (including cloud restore removals) must not imply keeping cache.
			require.NoError(t, GetStoreManager().uninstallLocked(context.Background(), instance, true, preserveCache, nil))
			if preserveCache {
				data, err := os.ReadFile(cacheFile)
				require.NoError(t, err)
				assert.Equal(t, "cached icons", string(data))
			} else {
				_, err := os.Stat(folder)
				assert.True(t, os.IsNotExist(err))
			}
		})
	}
}
