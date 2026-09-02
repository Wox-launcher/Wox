package plugin

import (
	"testing"
	"wox/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyPluginArtifact(t *testing.T) {
	kind, err := ClassifyPluginArtifact(PLUGIN_RUNTIME_PYTHON, "https://example.com/plugin.py?version=1")
	require.NoError(t, err)
	assert.Equal(t, PluginArtifactSingleFile, kind)

	kind, err = ClassifyPluginArtifact(PLUGIN_RUNTIME_NODEJS, "https://example.com/plugin.js?token=x")
	require.NoError(t, err)
	assert.Equal(t, PluginArtifactSingleFile, kind)

	kind, err = ClassifyPluginArtifact(PLUGIN_RUNTIME_PYTHON, "https://example.com/plugin.wox")
	require.NoError(t, err)
	assert.Equal(t, PluginArtifactPackage, kind)

	kind, err = ClassifyPluginArtifact(PLUGIN_RUNTIME_SCRIPT, "https://example.com/Wox.Plugin.Script.Timestamp.py")
	require.NoError(t, err)
	assert.Equal(t, PluginArtifactScript, kind)

	kind, err = ClassifyPluginArtifact(PLUGIN_RUNTIME_SCRIPT, "https://example.com/Wox.Plugin.Script.UUID")
	require.NoError(t, err)
	assert.Equal(t, PluginArtifactScript, kind)
	require.NoError(t, ValidateStorePluginManifest(StorePluginManifest{
		Runtime:     PLUGIN_RUNTIME_SCRIPT,
		DownloadUrl: "https://example.com/Wox.Plugin.Script.UUID",
	}))

	_, err = ClassifyPluginArtifact(PLUGIN_RUNTIME_PYTHON, "https://example.com/plugin")
	require.Error(t, err)

	_, err = ClassifyPluginArtifact(PLUGIN_RUNTIME_PYTHON, "https://example.com/plugin.js")
	require.Error(t, err)

	_, err = ClassifyPluginArtifact(PLUGIN_RUNTIME_NODEJS, "https://example.com/plugin.py")
	require.Error(t, err)

	_, err = ClassifyPluginArtifact(PLUGIN_RUNTIME_SCRIPT, "https://example.com/plugin.wox")
	require.Error(t, err)

	_, err = ClassifyPluginArtifact(PLUGIN_RUNTIME_NODEJS, "https://example.com/plugin.exe")
	require.Error(t, err)
}

func TestValidateStorePluginManifestRequiresSingleFileMinWoxVersion(t *testing.T) {
	err := ValidateStorePluginManifest(StorePluginManifest{
		Id:          "id",
		Runtime:     PLUGIN_RUNTIME_PYTHON,
		DownloadUrl: "https://example.com/plugin.py",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "MinWoxVersion")

	err = ValidateStorePluginManifest(StorePluginManifest{
		Id:            "id",
		Runtime:       PLUGIN_RUNTIME_PYTHON,
		DownloadUrl:   "https://example.com/plugin.py",
		MinWoxVersion: "2.0.0",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "below")

	err = ValidateStorePluginManifest(StorePluginManifest{
		Id:            "id",
		Runtime:       PLUGIN_RUNTIME_PYTHON,
		DownloadUrl:   "https://example.com/plugin.py",
		MinWoxVersion: SingleFilePluginMinWoxVersion,
	})
	require.NoError(t, err)
}

func TestValidateSingleFileHeaderMatchesManifest(t *testing.T) {
	manifest := StorePluginManifest{Id: "abc", Runtime: PLUGIN_RUNTIME_PYTHON, Version: "1.2.3"}
	err := ValidateSingleFileHeaderMatchesManifest(Metadata{Id: "abc", Runtime: "PYTHON", Version: "1.2.3"}, manifest)
	require.NoError(t, err)

	err = ValidateSingleFileHeaderMatchesManifest(Metadata{Id: "other", Runtime: "PYTHON", Version: "1.2.3"}, manifest)
	require.Error(t, err)

	err = ValidateSingleFileHeaderMatchesManifest(Metadata{Id: "abc", Runtime: "NODEJS", Version: "1.2.3"}, manifest)
	require.Error(t, err)

	err = ValidateSingleFileHeaderMatchesManifest(Metadata{Id: "abc", Runtime: "PYTHON", Version: "9.9.9"}, manifest)
	require.Error(t, err)
}

func TestPluginEntryMode(t *testing.T) {
	assert.Equal(t, PluginEntryModePackage, PluginEntryMode(Metadata{Directory: "/tmp/plugin@1.0.0"}))

	initSingleFileTestLocation(t)
	assert.Equal(t, PluginEntryModeSingleFile, PluginEntryMode(Metadata{
		Directory: util.GetLocation().GetUserSingleFilePluginsDirectory(),
		Entry:     "plugin.py",
	}))
}
