package plugin

import (
	"context"
	"os"
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
