package app

import (
	"context"
	"testing"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"

	"github.com/stretchr/testify/require"
)

type emptyAPIImpl struct {
}

func (e emptyAPIImpl) OnGetDynamicSetting(context.Context, func(string) definition.PluginSettingDefinitionItem) {
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

func (e emptyAPIImpl) OnSettingChanged(ctx context.Context, callback func(key string, value string)) {
}

func (e emptyAPIImpl) OnDeepLink(ctx context.Context, callback func(arguments map[string]string)) {
}

func (e emptyAPIImpl) OnUnload(ctx context.Context, callback func()) {
}

func (e emptyAPIImpl) RegisterQueryCommands(ctx context.Context, commands []plugin.MetadataCommand) {
}

func (e emptyAPIImpl) AIChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, callback common.ChatStreamFunc) error {
	return nil
}

func (e emptyAPIImpl) OnMRURestore(ctx context.Context, callback func(mruData plugin.MRUData) (*plugin.QueryResult, error)) {
}

func (e emptyAPIImpl) UpdateResult(ctx context.Context, result plugin.UpdatableResult) bool {
	return false
}

func (e emptyAPIImpl) GetUpdatableResult(ctx context.Context, resultId string) *plugin.UpdatableResult {
	return nil
}

func (e emptyAPIImpl) IsVisible(ctx context.Context) bool {
	return false
}

func TestMacRetriever_ParseAppInfo(t *testing.T) {
	if util.IsMacOS() {
		util.GetLocation().Init()
		appRetriever.UpdateAPI(emptyAPIImpl{})
		_, err := appRetriever.ParseAppInfo(nil, "/System/Applications/Siri.app")
		require.NoError(t, err)
	}
}
