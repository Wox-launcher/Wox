package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"wox/common"
	"wox/plugin"
	"wox/plugin/plugintest"
	"wox/util/clipboard"

	"github.com/stretchr/testify/require"
)

type localSendTestAPI struct {
	plugintest.StubAPI
	info    plugin.GetPluginInfoResult
	query   common.PlainQuery
	message string
}

func (a *localSendTestAPI) GetPluginInfo(_ context.Context, option plugin.GetPluginInfoOption) plugin.GetPluginInfoResult {
	if option.PluginId == localSendPluginID {
		return a.info
	}
	return plugin.GetPluginInfoResult{}
}

func (a *localSendTestAPI) ChangeQuery(_ context.Context, query common.PlainQuery) {
	a.query = query
}

func (a *localSendTestAPI) ShowToastMsg(_ context.Context, option plugin.ShowToastMsgOption) plugin.ShowToastMsgResult {
	a.message = option.Message
	return plugin.ShowToastMsgResult{}
}

func TestLocalSendActionOpensAnOrdinaryInputQuery(t *testing.T) {
	image := filepath.Join(t.TempDir(), "剪贴板 image.png")
	require.NoError(t, os.WriteFile(image, []byte("original image"), 0o600))
	record := ClipboardRecord{ID: "image-record", Type: string(clipboard.ClipboardTypeImage), FilePath: image}
	info := plugin.GetPluginInfoResult{
		Installed: true, Enabled: true, Ready: true, Version: "0.1.1",
		TriggerKeywords: []string{"*", "receive", "share"},
	}
	api := &localSendTestAPI{info: info}
	c := &ClipboardPlugin{api: api}

	var result plugin.QueryResult
	old := info
	old.Version = "0.1.0"
	c.appendLocalSendAction(&result, record, func() localSendTarget { return localSendTargetFromInfo(old) })
	require.Empty(t, result.Actions)
	icon := common.NewWoxImageAbsolutePath(image)
	info.Icon = &icon
	c.appendLocalSendAction(&result, record, func() localSendTarget { return localSendTargetFromInfo(info) })
	require.Len(t, result.Actions, 1)
	action := result.Actions[0]
	require.Equal(t, icon, action.Icon)
	require.True(t, action.PreventHideAfterAction)
	require.False(t, action.IsDefault)

	action.Action(t.Context(), plugin.ActionContext{})
	require.Equal(t, plugin.QueryTypeInput, api.query.QueryType)
	require.Equal(t, "share ", api.query.QueryText)
	require.True(t, api.query.QueryScope.IsEmpty())
	require.Equal(t, image, api.query.ContextData[localSendFilePathKey])
	require.Equal(t, localSendPluginID, api.query.ContextData[plugin.QueryContextDataRecipientKey])

	require.NoError(t, os.Remove(image))
	action.Action(t.Context(), plugin.ActionContext{})
	require.Equal(t, "i18n:plugin_clipboard_localsend_image_missing", api.message)
	api.info.Ready = false
	api.message = ""
	api.query = common.PlainQuery{}
	action.Action(t.Context(), plugin.ActionContext{})
	require.Empty(t, api.message)
	require.True(t, api.query.IsEmpty())
}

func TestLocalSendTextActionPreservesOriginalText(t *testing.T) {
	text := "  第一行\r\nsecond line 🙂\n  "
	record := ClipboardRecord{ID: "text-record", Type: string(clipboard.ClipboardTypeText), Content: text}
	api := &localSendTestAPI{info: plugin.GetPluginInfoResult{
		Installed: true, Enabled: true, Ready: true, Version: "0.1.1",
		TriggerKeywords: []string{"send"},
	}}
	c := &ClipboardPlugin{api: api}
	lookup := func() localSendTarget { return localSendTargetFromInfo(api.info) }
	var result plugin.QueryResult
	c.appendLocalSendAction(&result, record, lookup)
	require.Empty(t, result.Actions)

	api.info.Version = "0.1.2"
	c.appendLocalSendAction(&result, record, lookup)
	require.Len(t, result.Actions, 1)
	result.Actions[0].Action(t.Context(), plugin.ActionContext{})
	require.Equal(t, "send ", api.query.QueryText)
	require.True(t, api.query.QueryScope.IsEmpty())
	require.Equal(t, common.ContextData{
		plugin.QueryContextDataRecipientKey: localSendPluginID,
		localSendTextKey:                    text,
	}, api.query.ContextData)
	require.Empty(t, api.message)
}
