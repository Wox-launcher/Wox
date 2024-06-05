package app

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"wox/plugin"
	"wox/plugin/llm"
	"wox/setting/definition"
	"wox/share"
	"wox/util"
)

type emptyAPIImpl struct {
}

func (e emptyAPIImpl) OnGetDynamicSetting(ctx context.Context, callback func(key string) definition.PluginSettingDefinitionItem) {
}

func (e emptyAPIImpl) ChangeQuery(ctx context.Context, query share.PlainQuery) {
}

func (e emptyAPIImpl) HideApp(ctx context.Context) {
}

func (e emptyAPIImpl) ShowApp(ctx context.Context) {
}

func (e emptyAPIImpl) Notify(ctx context.Context, title string, description string) {
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

func (e emptyAPIImpl) RegisterQueryCommands(ctx context.Context, commands []plugin.MetadataCommand) {
}

func (e emptyAPIImpl) LLMChatStream(ctx context.Context, conversations []llm.Conversation, callback llm.ChatStreamFunc) error {
	return nil
}

func TestMacRetriever_ParseAppInfo(t *testing.T) {
	if util.IsMacOS() {
		appRetriever.UpdateAPI(emptyAPIImpl{})
		_, err := appRetriever.ParseAppInfo(nil, "/System/Applications/Siri.app")
		require.NoError(t, err)
	}
}
