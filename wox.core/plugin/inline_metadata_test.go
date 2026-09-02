package plugin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wox/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initSingleFileTestLocation(t *testing.T) {
	t.Helper()
	if logger == nil {
		logger = util.CreateLogger(filepath.Join(os.TempDir(), "wox-plugin-tests"))
	}
	dataDir := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, dataDir)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(dataDir, "user"))
	require.NoError(t, util.GetLocation().Init())
}

func writeTempPluginFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

func TestParseInlineMetadataWithBracesInStrings(t *testing.T) {
	path := writeTempPluginFile(t, "weather.py", `# {
#   "Id": "com.example.weather",
#   "Name": "Weather",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.4.2",
#   "Runtime": "PYTHON",
#   "TriggerKeywords": ["weather"],
#   "Description": "Use {query} and } braces"
# }

plugin = object()
`)
	parsed, err := parseInlineMetadata(path)
	require.NoError(t, err)
	assert.Equal(t, "com.example.weather", parsed.Metadata.Id)
	assert.Equal(t, "Use {query} and } braces", parsed.Metadata.GetDescription(context.Background()))
	assert.True(t, parsed.hasKey("Runtime"))
	assert.False(t, parsed.hasKey("Entry"))
}

func TestParseInlineMetadataPrettyPrintedAndShebang(t *testing.T) {
	path := writeTempPluginFile(t, "hello.js", `#!/usr/bin/env node
// {
//   "Id": "com.example.hello",
//   "Name": "Hello",
//   "Version": "1.0.0",
//   "MinWoxVersion": "2.4.2",
//   "Runtime": "NODEJS",
//   "TriggerKeywords": ["hello"]
// }

module.exports.plugin = {}
`)
	parsed, err := parseInlineMetadata(path)
	require.NoError(t, err)
	assert.Equal(t, "com.example.hello", parsed.Metadata.Id)
	assert.Equal(t, "NODEJS", parsed.Metadata.Runtime)
}

func TestParseInlineMetadataEscapedQuotes(t *testing.T) {
	path := writeTempPluginFile(t, "quotes.py", `# {"Id":"id","Name":"Say \"hi\"","Version":"1.0.0","MinWoxVersion":"2.4.2","Runtime":"PYTHON","TriggerKeywords":["q"]}
plugin = 1
`)
	parsed, err := parseInlineMetadata(path)
	require.NoError(t, err)
	assert.Equal(t, `Say "hi"`, parsed.Metadata.GetName(context.Background()))
}

func TestParseSingleFilePluginMetadataRejectsEntry(t *testing.T) {
	initSingleFileTestLocation(t)
	path := writeTempPluginFile(t, "bad.py", `# {
#   "Id": "id",
#   "Name": "Bad",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.4.2",
#   "Runtime": "PYTHON",
#   "Entry": "other.py",
#   "TriggerKeywords": ["bad"]
# }
plugin = 1
`)
	_, err := (&Manager{}).ParseSingleFilePluginMetadata(context.Background(), path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Entry")
}

func TestParseSingleFilePluginMetadataRejectsRuntimeMismatch(t *testing.T) {
	initSingleFileTestLocation(t)
	path := writeTempPluginFile(t, "bad.py", `# {
#   "Id": "id",
#   "Name": "Bad",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.4.2",
#   "Runtime": "NODEJS",
#   "TriggerKeywords": ["bad"]
# }
plugin = 1
`)
	_, err := (&Manager{}).ParseSingleFilePluginMetadata(context.Background(), path)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "runtime")
}

func TestParseSingleFilePluginMetadataRequiresFieldsAndSetsLocation(t *testing.T) {
	initSingleFileTestLocation(t)
	path := writeTempPluginFile(t, "weather.py", `# {
#   "Id": "com.example.weather",
#   "Name": "Weather",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.4.2",
#   "Runtime": "PYTHON",
#   "Icon": "emoji:🌤️",
#   "TriggerKeywords": ["weather"],
#   "I18n": {"en_US": {"hello": "Hello"}}
# }
plugin = 1
`)
	metadata, err := (&Manager{}).ParseSingleFilePluginMetadata(context.Background(), path)
	require.NoError(t, err)
	assert.Equal(t, "weather.py", metadata.Entry)
	assert.True(t, IsSingleFilePlugin(metadata))
	assert.Equal(t, "Hello", metadata.I18n["en_US"]["hello"])
}

func TestParseSingleFilePluginMetadataRejectsRelativeIcon(t *testing.T) {
	initSingleFileTestLocation(t)
	path := writeTempPluginFile(t, "icon.py", `# {
#   "Id": "id",
#   "Name": "Icon",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.4.2",
#   "Runtime": "PYTHON",
#   "Icon": "relative:icon.png",
#   "TriggerKeywords": ["icon"]
# }
plugin = 1
`)
	_, err := (&Manager{}).ParseSingleFilePluginMetadata(context.Background(), path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "relative")
}

func TestParseSingleFilePluginMetadataDoesNotLoadSharedLang(t *testing.T) {
	initSingleFileTestLocation(t)
	langDir := filepath.Join(util.GetLocation().GetUserSingleFilePluginsDirectory(), "lang")
	require.NoError(t, os.MkdirAll(langDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(langDir, "en_US.json"), []byte(`{"shared":"nope"}`), 0644))
	path := writeTempPluginFile(t, "i18n.py", `# {
#   "Id": "id",
#   "Name": "I18n",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.4.2",
#   "Runtime": "PYTHON",
#   "TriggerKeywords": ["i18n"],
#   "I18n": {"en_US": {"hello": "Hello"}}
# }
plugin = 1
`)
	metadata, err := (&Manager{}).ParseSingleFilePluginMetadata(context.Background(), path)
	require.NoError(t, err)
	assert.Equal(t, "Hello", metadata.I18n["en_US"]["hello"])
	assert.Empty(t, metadata.I18n["en_US"]["shared"])
}

func TestParseScriptMetadataStillWorksWithoutRuntime(t *testing.T) {
	path := writeTempPluginFile(t, "script.py", `# {
#   "Id": "script-id",
#   "Name": "Script",
#   "TriggerKeywords": ["s"]
# }
print("hi")
`)
	metadata, err := (&Manager{}).ParseScriptMetadata(context.Background(), path)
	require.NoError(t, err)
	assert.Equal(t, string(PLUGIN_RUNTIME_SCRIPT), metadata.Runtime)
	assert.Equal(t, "script.py", metadata.Entry)
}

func TestParseScriptMetadataRejectsPythonRuntime(t *testing.T) {
	initSingleFileTestLocation(t)
	path := writeTempPluginFile(t, "moved.py", `# {
#   "Id": "id",
#   "Name": "Moved",
#   "Runtime": "PYTHON",
#   "TriggerKeywords": ["m"]
# }
plugin = 1
`)
	_, err := (&Manager{}).ParseScriptMetadata(context.Background(), path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "single-file")
}

func TestIsReservedUserPluginDirectory(t *testing.T) {
	assert.True(t, IsReservedUserPluginDirectory("scripts"))
	assert.True(t, IsReservedUserPluginDirectory("single-file"))
	assert.False(t, IsReservedUserPluginDirectory("com.example@1.0.0"))
}

func TestShouldIgnoreSingleFileWatchName(t *testing.T) {
	assert.True(t, shouldIgnoreSingleFileWatchName(".hidden.py"))
	assert.True(t, shouldIgnoreSingleFileWatchName("foo.py~"))
	assert.True(t, shouldIgnoreSingleFileWatchName("foo.py.tmp"))
	assert.False(t, shouldIgnoreSingleFileWatchName("Wox.Plugin.Weather.py"))
}
