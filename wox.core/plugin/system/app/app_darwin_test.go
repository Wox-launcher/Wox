package app

import (
	"context"
	"testing"
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

func (e emptyAPIImpl) OnSettingChanged(ctx context.Context, callback func(context.Context, string, string)) {
}

func (e emptyAPIImpl) OnDeepLink(ctx context.Context, callback func(context.Context, map[string]string)) {
}

func (e emptyAPIImpl) OnUnload(ctx context.Context, callback func(context.Context)) {
}

func (e emptyAPIImpl) RegisterQueryCommands(ctx context.Context, commands []plugin.MetadataCommand) {
}

func (e emptyAPIImpl) AIChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, callback common.ChatStreamFunc) error {
	return nil
}

func (e emptyAPIImpl) OnMRURestore(ctx context.Context, callback func(context.Context, plugin.MRUData) (*plugin.QueryResult, error)) {
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

func (e emptyAPIImpl) Copy(ctx context.Context, params plugin.CopyParams) {
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
