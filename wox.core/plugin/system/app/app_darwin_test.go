package app

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"
	"wox/util/fileicon"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type emptyAPIImpl struct {
}

func (e emptyAPIImpl) OnGetDynamicSetting(
	context.Context,
	func(context.Context, string) definition.PluginSettingDefinitionItem,
) {
}

func (e emptyAPIImpl) ChangeQuery(ctx context.Context, query common.PlainQuery) {
}

func (e emptyAPIImpl) HideApp(ctx context.Context) {
}

func (e emptyAPIImpl) ShowApp(ctx context.Context) {
}

func (e emptyAPIImpl) Notify(ctx context.Context, message string) {
}

func (e emptyAPIImpl) PushAttention(ctx context.Context, request plugin.PushAttentionRequest) {
}

func (e emptyAPIImpl) Log(ctx context.Context, level plugin.LogLevel, msg string) {
}

func (e emptyAPIImpl) GetTranslation(ctx context.Context, key string) string {
	return ""
}

func (e emptyAPIImpl) GetSetting(ctx context.Context, key string) string {
	return ""
}

func (e emptyAPIImpl) SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool) {
}

func (e emptyAPIImpl) SetSetting(ctx context.Context, option plugin.SetSettingOption) plugin.SetSettingResult {
	return plugin.SetSettingResult{Success: true}
}

func (e emptyAPIImpl) OnSettingChanged(ctx context.Context, callback func(context.Context, string, string)) {
}

func (e emptyAPIImpl) OnDeepLink(ctx context.Context, callback func(context.Context, map[string]string)) {
}

func (e emptyAPIImpl) OnUnload(ctx context.Context, callback func(context.Context)) {
}

func (e emptyAPIImpl) ShowToolbarMsg(ctx context.Context, msg plugin.ToolbarMsg) {
}

func (e emptyAPIImpl) ClearToolbarMsg(ctx context.Context, toolbarMsgId string) {
}

func (e emptyAPIImpl) OnEnterPluginQuery(ctx context.Context, callback func(context.Context)) {
}

func (e emptyAPIImpl) OnLeavePluginQuery(ctx context.Context, callback func(context.Context)) {
}

func (e emptyAPIImpl) RegisterQueryCommands(ctx context.Context, commands []plugin.MetadataCommand) {
}

func (e emptyAPIImpl) AIChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, callback common.ChatStreamFunc) error {
	return nil
}

func (e emptyAPIImpl) OnMRURestore(ctx context.Context, callback func(context.Context, plugin.MRUData) (*plugin.QueryResult, error)) {
}

func (e emptyAPIImpl) OnHandlePluginCommand(ctx context.Context, handler plugin.PluginCommandHandler) {
}

func (e emptyAPIImpl) InvokePluginCommand(ctx context.Context, request plugin.PluginCommandRequest) (plugin.PluginCommandResult, error) {
	return plugin.PluginCommandResult{}, nil
}

func (e emptyAPIImpl) UpdateResult(ctx context.Context, result plugin.UpdatableResult) bool {
	return false
}

func (e emptyAPIImpl) PushResults(ctx context.Context, query plugin.Query, results []plugin.QueryResult) bool {
	return false
}

func (e emptyAPIImpl) GetUpdatableResult(ctx context.Context, resultId string) *plugin.UpdatableResult {
	return nil
}

func (e emptyAPIImpl) IsVisible(ctx context.Context) bool {
	return false
}

func (e emptyAPIImpl) RefreshQuery(ctx context.Context, params plugin.RefreshQueryParam) {
}

func (e emptyAPIImpl) RefreshGlance(ctx context.Context, ids []string) {
}

func (e emptyAPIImpl) Copy(ctx context.Context, params plugin.CopyParams) {
}

func (e emptyAPIImpl) Screenshot(ctx context.Context, option plugin.ScreenshotOption) plugin.ScreenshotResult {
	return plugin.ScreenshotResult{}
}

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
