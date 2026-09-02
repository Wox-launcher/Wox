package util

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initPluginCacheTestLocation(t *testing.T) {
	t.Helper()
	dataDir := t.TempDir()
	t.Setenv(TestWoxDataDirEnv, dataDir)
	t.Setenv(TestUserDataDirEnv, filepath.Join(dataDir, "user"))
	require.NoError(t, GetLocation().Init())
}

func TestPluginCacheDirectoryUsesPluginID(t *testing.T) {
	initPluginCacheTestLocation(t)

	pluginID := "a8c4e2b1-6d3f-4a19-9e7c-2b5f0d8a1c63"
	folder, err := GetLocation().GetPluginCacheDirectory(pluginID)
	require.NoError(t, err)
	assert.Equal(t, path.Join(GetLocation().GetCacheDirectory(), "plugins", pluginID), folder)
}

func TestEnsurePluginCacheDirectoryCreatesFolder(t *testing.T) {
	initPluginCacheTestLocation(t)

	folder, err := GetLocation().EnsurePluginCacheDirectory("gif-search")
	require.NoError(t, err)
	info, statErr := os.Stat(folder)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

func TestRemovePluginCacheDirectoryDeletesFolder(t *testing.T) {
	initPluginCacheTestLocation(t)

	folder, err := GetLocation().EnsurePluginCacheDirectory("to-remove")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path.Join(folder, "download.gif"), []byte("gif"), 0644))

	require.NoError(t, GetLocation().RemovePluginCacheDirectory("to-remove"))
	_, statErr := os.Stat(folder)
	assert.True(t, os.IsNotExist(statErr))
}

func TestRemovePluginCacheDirectoryIgnoresMissingFolder(t *testing.T) {
	initPluginCacheTestLocation(t)
	require.NoError(t, GetLocation().RemovePluginCacheDirectory("never-created"))
}

func TestPluginCacheDirectoryRejectsUnsafeIDs(t *testing.T) {
	initPluginCacheTestLocation(t)

	for _, pluginID := range []string{"", "../escape", "foo/bar", `foo\bar`, "foo/../bar"} {
		_, err := GetLocation().GetPluginCacheDirectory(pluginID)
		assert.Error(t, err, pluginID)
	}
}
