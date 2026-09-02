package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wox/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnloadSingleFilePluginByPathIgnoresIDChange(t *testing.T) {
	initSingleFileTestLocation(t)
	singleFileDir, err := filepath.Abs(util.GetLocation().GetUserSingleFilePluginsDirectory())
	require.NoError(t, err)
	manager := &Manager{instances: []*Instance{{
		Metadata: Metadata{
			Id:        "old-id",
			Name:      "Old",
			Runtime:   string(PLUGIN_RUNTIME_PYTHON),
			Directory: singleFileDir,
			Entry:     "Wox.Plugin.Weather.py",
		},
	}}}

	manager.unloadSingleFilePluginByPath(context.Background(), filepath.Join(singleFileDir, "Wox.Plugin.Weather.py"))
	assert.Empty(t, manager.pluginInstancesSnapshot())
}

func TestReloadSingleFilePluginKeepsInstanceWhenMetadataInvalid(t *testing.T) {
	initSingleFileTestLocation(t)
	singleFileDir, err := filepath.Abs(util.GetLocation().GetUserSingleFilePluginsDirectory())
	require.NoError(t, err)
	filePath := filepath.Join(singleFileDir, "keep.py")
	require.NoError(t, os.WriteFile(filePath, []byte("not a plugin"), 0644))
	manager := &Manager{instances: []*Instance{{
		Metadata: Metadata{
			Id:        "keep-id",
			Name:      "Keep",
			Runtime:   string(PLUGIN_RUNTIME_PYTHON),
			Directory: singleFileDir,
			Entry:     "keep.py",
		},
	}}}
	manager.reloadSingleFilePlugin(context.Background(), filePath, "invalid metadata")
	require.Len(t, manager.pluginInstancesSnapshot(), 1)
	assert.Equal(t, "keep-id", manager.pluginInstancesSnapshot()[0].Metadata.Id)
}

func TestSingleFilePluginIDConflictKeepsOtherPlugin(t *testing.T) {
	initSingleFileTestLocation(t)
	singleFileDir := util.GetLocation().GetUserSingleFilePluginsDirectory()
	manager := &Manager{instances: []*Instance{{
		Metadata: Metadata{Id: "duplicate", Directory: filepath.Join(t.TempDir(), "packaged")},
	}}}

	err := manager.ensureSingleFilePluginIDAvailable(Metadata{
		Id:        "DUPLICATE",
		Directory: singleFileDir,
		Entry:     "duplicate.py",
	})
	require.Error(t, err)
	require.Len(t, manager.pluginInstancesSnapshot(), 1)
}

func TestIgnoreSingleFileWatchExpires(t *testing.T) {
	manager := &Manager{singleFileWatchIgnored: util.NewHashMap[string, int64]()}
	filePath := filepath.Join(t.TempDir(), "plugin.py")
	manager.IgnoreSingleFileWatch(filePath)
	assert.True(t, manager.isSingleFileWatchIgnored(filePath))
	manager.singleFileWatchIgnored.Store(filePath, time.Now().Add(-time.Millisecond).UnixMilli())
	assert.False(t, manager.isSingleFileWatchIgnored(filePath))
}

func TestReadFileWithRetryReturnsStableContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.py")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0644))
	data, err := readFileWithRetry(path, 200*time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestDebounceKeyedTimerRunsOnce(t *testing.T) {
	timers := util.NewHashMap[string, *time.Timer]()
	runs := 0
	debounceKeyedTimer(timers, "key", 30*time.Millisecond, func() { runs++ })
	debounceKeyedTimer(timers, "key", 30*time.Millisecond, func() { runs++ })
	time.Sleep(80 * time.Millisecond)
	assert.Equal(t, 1, runs)
}
