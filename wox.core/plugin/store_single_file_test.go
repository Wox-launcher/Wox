package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"wox/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUninstallSingleFilePluginRemovesOnlyTargetFile(t *testing.T) {
	initSingleFileTestLocation(t)
	directory := util.GetLocation().GetUserSingleFilePluginsDirectory()
	target := filepath.Join(directory, "keep.py")
	other := filepath.Join(directory, "other.py")
	require.NoError(t, os.WriteFile(target, []byte("plugin = 1\n"), 0644))
	require.NoError(t, os.WriteFile(other, []byte("plugin = 2\n"), 0644))

	instance := &Instance{
		Metadata: Metadata{
			Id:        "keep-id",
			Runtime:   string(PLUGIN_RUNTIME_PYTHON),
			Directory: directory,
			Entry:     "keep.py",
		},
	}
	err := GetStoreManager().uninstallSingleFilePlugin(context.Background(), instance)
	require.NoError(t, err)

	info, statErr := os.Stat(directory)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
	assert.True(t, util.IsFileExists(other))
}

func TestUninstallSingleFilePluginRejectsPathEscape(t *testing.T) {
	initSingleFileTestLocation(t)
	instance := &Instance{
		Metadata: Metadata{
			Id:        "escape",
			Runtime:   string(PLUGIN_RUNTIME_PYTHON),
			Directory: util.GetLocation().GetUserSingleFilePluginsDirectory(),
			Entry:     filepath.Join("..", "secrets.py"),
		},
	}
	err := GetStoreManager().uninstallSingleFilePlugin(context.Background(), instance)
	require.Error(t, err)
}

func TestInstallSingleFilePluginRejectsHeaderManifestMismatch(t *testing.T) {
	initSingleFileTestLocation(t)
	content := `# {
#   "Id": "com.example.other",
#   "Name": "Store",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.4.2",
#   "Runtime": "PYTHON",
#   "TriggerKeywords": ["store"]
# }
plugin = object()
`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(content))
	}))
	t.Cleanup(server.Close)

	err := GetStoreManager().installSingleFilePluginWithProgress(context.Background(), StorePluginManifest{
		Id:            "com.example.store",
		Name:          "Store",
		Version:       "1.0.0",
		MinWoxVersion: SingleFilePluginMinWoxVersion,
		Runtime:       PLUGIN_RUNTIME_PYTHON,
		DownloadUrl:   server.URL + "/Wox.Plugin.Store.py",
	}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Id")

	directory := util.GetLocation().GetUserSingleFilePluginsDirectory()
	assert.False(t, util.IsFileExists(filepath.Join(directory, "Wox.Plugin.Store.py")))
	info, statErr := os.Stat(directory)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
}

func TestInstallSingleFilePluginRejectsRuntimeMismatch(t *testing.T) {
	initSingleFileTestLocation(t)
	content := `// {
//   "Id": "com.example.store",
//   "Name": "Store",
//   "Version": "1.0.0",
//   "MinWoxVersion": "2.4.2",
//   "Runtime": "NODEJS",
//   "TriggerKeywords": ["store"]
// }
module.exports.plugin = {}
`
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte(content))
	}))
	t.Cleanup(server.Close)

	err := GetStoreManager().installSingleFilePluginWithProgress(context.Background(), StorePluginManifest{
		Id:            "com.example.store",
		Name:          "Store",
		Version:       "1.0.0",
		MinWoxVersion: SingleFilePluginMinWoxVersion,
		Runtime:       PLUGIN_RUNTIME_PYTHON,
		DownloadUrl:   server.URL + "/Wox.Plugin.Store.py",
	}, nil)
	require.Error(t, err)
}

func TestInstalledPluginArtifactKind(t *testing.T) {
	initSingleFileTestLocation(t)
	packageInstance := &Instance{Metadata: Metadata{Id: "id", Runtime: string(PLUGIN_RUNTIME_PYTHON), Directory: "/tmp/id@1.0.0"}}
	assert.Equal(t, PluginArtifactPackage, InstalledPluginArtifactKind(packageInstance))

	singleFileInstance := &Instance{Metadata: Metadata{
		Id:        "id",
		Runtime:   string(PLUGIN_RUNTIME_PYTHON),
		Directory: util.GetLocation().GetUserSingleFilePluginsDirectory(),
		Entry:     "plugin.py",
	}}
	assert.Equal(t, PluginArtifactSingleFile, InstalledPluginArtifactKind(singleFileInstance))
}
