package browserbookmark

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

func getLocalFirefoxRootDirsForTest() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"~/Library/Application Support/Firefox"}
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return nil
		}
		return []string{filepath.Join(appData, "Mozilla", "Firefox")}
	case "linux":
		return []string{
			"~/.mozilla/firefox",
			"~/snap/firefox/common/.mozilla/firefox",
			"~/.var/app/org.mozilla.firefox/.mozilla/firefox",
		}
	default:
		return nil
	}
}

func TestBrowserBookmarkPlugin_LoadFirefoxBookmarksFromLocalProfile(t *testing.T) {
	plugin := &BrowserBookmarkPlugin{api: &mockAPI{}}
	rootDirs := getLocalFirefoxRootDirsForTest()
	if len(rootDirs) == 0 {
		t.Skip("no local Firefox root directories for current OS")
	}

	profileDirs := plugin.resolveFirefoxProfileDirs(context.Background(), rootDirs, "Firefox")
	if len(profileDirs) == 0 {
		t.Skipf("no Firefox profiles found under roots: %v", rootDirs)
	}

	t.Logf("Firefox roots: %v", rootDirs)
	t.Logf("Firefox profiles: %v", profileDirs)

	bookmarks := plugin.loadFirefoxBookmarks(context.Background())
	t.Logf("Loaded %d Firefox bookmarks", len(bookmarks))
	for i, bookmark := range bookmarks {
		if i >= 10 {
			break
		}
		t.Logf("[%d] %s -> %s", i+1, bookmark.Name, bookmark.Url)
	}

	assert.Greater(t, len(bookmarks), 0, "expected at least one local Firefox bookmark")
}

// Mock API for testing
type mockAPI struct{}

func (m *mockAPI) Log(ctx context.Context, level plugin.LogLevel, msg string) {
	// Output log messages during testing
	if testing.Verbose() {
		println("LOG:", msg)
	}
}
func (m *mockAPI) Notify(ctx context.Context, msg string)                {}
func (m *mockAPI) GetTranslation(ctx context.Context, key string) string { return key }
func (m *mockAPI) GetSetting(ctx context.Context, key string) string     { return "" }
func (m *mockAPI) SaveSetting(ctx context.Context, key string, value string, isGlobal bool) {
}
func (m *mockAPI) GetAllSettings(ctx context.Context) map[string]string     { return nil }
func (m *mockAPI) OpenSettingDialog(ctx context.Context)                    {}
func (m *mockAPI) HideApp(ctx context.Context)                              {}
func (m *mockAPI) ShowApp(ctx context.Context)                              {}
func (m *mockAPI) ChangeQuery(ctx context.Context, query common.PlainQuery) {}
func (m *mockAPI) RestartApp(ctx context.Context)                           {}
func (m *mockAPI) ReloadPlugin(ctx context.Context, pluginId string)        {}
func (m *mockAPI) RemovePlugin(ctx context.Context, pluginId string)        {}
func (m *mockAPI) OnSettingChanged(ctx context.Context, callback func(ctx context.Context, key string, value string)) {
}
func (m *mockAPI) OnGetDynamicSetting(
	context.Context,
	func(context.Context, string) definition.PluginSettingDefinitionItem,
) {
}
func (m *mockAPI) OnDeepLink(ctx context.Context, callback func(context.Context, map[string]string)) {
}
func (m *mockAPI) OnUnload(ctx context.Context, callback func(context.Context))                 {}
func (m *mockAPI) RegisterQueryCommands(ctx context.Context, commands []plugin.MetadataCommand) {}
func (m *mockAPI) AIChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, callback common.ChatStreamFunc) error {
	return nil
}
func (m *mockAPI) OnMRURestore(
	ctx context.Context,
	callback func(context.Context, plugin.MRUData) (*plugin.QueryResult, error),
) {
}
func (m *mockAPI) UpdateResult(ctx context.Context, result plugin.UpdatableResult) bool {
	return false
}

func (m *mockAPI) PushResults(ctx context.Context, query plugin.Query, results []plugin.QueryResult) bool {
	return false
}

func (m *mockAPI) GetUpdatableResult(ctx context.Context, resultId string) *plugin.UpdatableResult {
	return nil
}

func (m *mockAPI) IsVisible(ctx context.Context) bool {
	return false
}

func (e *mockAPI) RefreshQuery(ctx context.Context, params plugin.RefreshQueryParam) {
}

func (m *mockAPI) Copy(ctx context.Context, params plugin.CopyParams) {
}
