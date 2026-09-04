package app

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
	"wox/util"
	"wox/util/fileicon"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMacRetriever_ParseAppInfo(t *testing.T) {
	if util.IsMacOS() {
		util.GetLocation().Init()
		appRetriever.UpdateAPI(emptyAPIImpl{})

		appPath := "/Applications/Visual Studio Code.app"
		fileicon.CleanFileIconCache(context.Background(), appPath)
		info, err := appRetriever.ParseAppInfo(nil, appPath)
		assert.False(t, info.IsDefaultIcon, "app should use a custom icon, not the default one")
		require.NoError(t, err)
	}
}

func TestApplicationPlugin_DeduplicateAppPaths(t *testing.T) {
	appPlugin := &ApplicationPlugin{retriever: &MacRetriever{}}
	paths := appPlugin.deduplicateAppPaths([]string{
		"/Applications/One.app",
		"/Applications/Two.app",
		"/Applications/One.app",
	})

	assert.Equal(t, []string{
		"/Applications/One.app",
		"/Applications/Two.app",
	}, paths)
}

func TestMacRetriever_LocalizedAppNames(t *testing.T) {
	appPath, _, _ := writeMacAppFixture(t)
	names := (&MacRetriever{}).getLocalizedAppNames(appPath)

	assert.Equal(t, []string{
		"Base Display",
		"Base Name",
		"한국어 이름",
		"Root Localized",
		"中文名称",
		"中文",
	}, names)
}

func TestApplicationPlugin_MacAppCacheFreshness(t *testing.T) {
	appPath, infoPlistPath, iconPath := writeMacAppFixture(t)
	retriever := &MacRetriever{}
	appPlugin := &ApplicationPlugin{retriever: retriever, api: emptyAPIImpl{}}
	fileInfo, err := os.Stat(appPath)
	require.NoError(t, err)
	iconInfo, err := os.Stat(iconPath)
	require.NoError(t, err)

	modifiedUnix := retriever.GetAppModifiedUnix(appPath, fileInfo)
	cached := appInfo{
		Path:                   appPath,
		Identity:               "com.example.fixture",
		LastModifiedUnix:       modifiedUnix,
		IconSourcePath:         iconPath,
		IconSourceModifiedUnix: iconInfo.ModTime().UnixNano(),
	}
	cache := map[string]appInfo{appPath: cached}
	_, reused := appPlugin.reuseAppFromCache(context.Background(), appPath, fileInfo, cache)
	assert.True(t, reused)

	require.NoError(t, os.WriteFile(infoPlistPath, []byte(macAppInfoPlist+" "), 0o644))
	_, reused = appPlugin.reuseAppFromCache(context.Background(), appPath, fileInfo, cache)
	assert.False(t, reused, "Info.plist changes must invalidate the app cache")

	cached.LastModifiedUnix = retriever.GetAppModifiedUnix(appPath, fileInfo)
	cache[appPath] = cached
	require.NoError(t, os.WriteFile(iconPath, []byte("changed icon bytes"), 0o644))
	_, reused = appPlugin.reuseAppFromCache(context.Background(), appPath, fileInfo, cache)
	assert.False(t, reused, "icon changes must invalidate the app cache")

	_, reused = appPlugin.reuseAppFromCache(context.Background(), filepath.Join(filepath.Dir(appPath), "New.app"), fileInfo, cache)
	assert.False(t, reused, "new apps must not reuse another cache entry")
}

// writeMacAppFixture creates the metadata variants consumed by macOS app indexing.
func writeMacAppFixture(t *testing.T) (string, string, string) {
	t.Helper()
	appPath := filepath.Join(t.TempDir(), "Fixture [Beta].app")
	resourcesPath := filepath.Join(appPath, "Contents", "Resources")
	localizedPath := filepath.Join(resourcesPath, "zh_CN.lproj")
	require.NoError(t, os.MkdirAll(localizedPath, 0o755))

	infoPlistPath := filepath.Join(appPath, "Contents", "Info.plist")
	require.NoError(t, os.WriteFile(infoPlistPath, []byte(macAppInfoPlist), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(resourcesPath, "InfoPlist.loctable"), []byte(macAppInfoPlistLoctable), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(resourcesPath, "InfoPlist.strings"), utf16LE(`
"CFBundleDisplayName" = "Root Localized";
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(localizedPath, "InfoPlist.strings"), []byte(`
"CFBundleDisplayName" = "中文名称";
"CFBundleName" = "中文";
`), 0o644))
	iconPath := filepath.Join(resourcesPath, "FixtureIcon.icns")
	require.NoError(t, os.WriteFile(iconPath, []byte("icon"), 0o644))
	return appPath, infoPlistPath, iconPath
}

func utf16LE(value string) []byte {
	data := []byte{0xff, 0xfe}
	for _, encoded := range utf16.Encode([]rune(value)) {
		data = binary.LittleEndian.AppendUint16(data, encoded)
	}
	return data
}

const macAppInfoPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDisplayName</key>
	<string>Base Display</string>
	<key>CFBundleName</key>
	<string>Base Name</string>
	<key>CFBundleIdentifier</key>
	<string>com.example.fixture</string>
	<key>CFBundleIconFile</key>
	<string>FixtureIcon</string>
</dict>
</plist>`

const macAppInfoPlistLoctable = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>ko</key>
	<dict>
		<key>CFBundleDisplayName</key>
		<string>한국어 이름</string>
	</dict>
	<key>LocProvenance</key>
	<string>ignored</string>
</dict>
</plist>`
