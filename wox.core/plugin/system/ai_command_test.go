package system

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util/overlay"
	"wox/util/selection"

	"github.com/stretchr/testify/require"
)

type aiCommandTestAPI struct {
	settings map[string]string

	mu           sync.Mutex
	streamCalls  int
	streamDone   chan struct{}
	streamEvents []common.ChatStreamData
	notifyCh     chan string
}

func newAICommandTestAPI(t *testing.T, commands []map[string]any) *aiCommandTestAPI {
	t.Helper()

	commandJSON, err := json.Marshal(commands)
	require.NoError(t, err)

	return &aiCommandTestAPI{
		settings:   map[string]string{"commands": string(commandJSON)},
		streamDone: make(chan struct{}, 1),
		notifyCh:   make(chan string, 10),
	}
}

func (a *aiCommandTestAPI) streamCallCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.streamCalls
}

func (a *aiCommandTestAPI) waitForNotification(t *testing.T) string {
	t.Helper()

	select {
	case notification := <-a.notifyCh:
		return notification
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for notification")
		return ""
	}
}

func (a *aiCommandTestAPI) ChangeQuery(ctx context.Context, query common.PlainQuery) {}
func (a *aiCommandTestAPI) HideApp(ctx context.Context)                              {}
func (a *aiCommandTestAPI) ShowApp(ctx context.Context)                              {}
func (a *aiCommandTestAPI) Notify(ctx context.Context, description string) {
	select {
	case a.notifyCh <- description:
	default:
	}
}
func (a *aiCommandTestAPI) PushAttention(ctx context.Context, request plugin.PushAttentionRequest) {}
func (a *aiCommandTestAPI) Log(ctx context.Context, level plugin.LogLevel, msg string)             {}
func (a *aiCommandTestAPI) GetTranslation(ctx context.Context, key string) string                  { return key }
func (a *aiCommandTestAPI) GetSetting(ctx context.Context, key string) string                      { return a.settings[key] }
func (a *aiCommandTestAPI) SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool) {
	a.settings[key] = value
}
func (a *aiCommandTestAPI) OnSettingChanged(ctx context.Context, callback func(ctx context.Context, key string, value string)) {
}
func (a *aiCommandTestAPI) OnGetDynamicSetting(ctx context.Context, callback func(ctx context.Context, key string) definition.PluginSettingDefinitionItem) {
}
func (a *aiCommandTestAPI) OnDeepLink(ctx context.Context, callback func(ctx context.Context, arguments map[string]string)) {
}
func (a *aiCommandTestAPI) OnUnload(ctx context.Context, callback func(ctx context.Context)) {
}
func (a *aiCommandTestAPI) OnMRURestore(ctx context.Context, callback func(ctx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error)) {
}
func (a *aiCommandTestAPI) ShowToolbarMsg(ctx context.Context, msg plugin.ToolbarMsg) {}
func (a *aiCommandTestAPI) ClearToolbarMsg(ctx context.Context, toolbarMsgId string)  {}
func (a *aiCommandTestAPI) OnEnterPluginQuery(ctx context.Context, callback func(ctx context.Context)) {
}
func (a *aiCommandTestAPI) OnLeavePluginQuery(ctx context.Context, callback func(ctx context.Context)) {
}
func (a *aiCommandTestAPI) RegisterQueryCommands(ctx context.Context, commands []plugin.MetadataCommand) {
}
func (a *aiCommandTestAPI) AIChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, callback common.ChatStreamFunc) error {
	a.mu.Lock()
	a.streamCalls++
	a.mu.Unlock()

	select {
	case a.streamDone <- struct{}{}:
	default:
	}

	if len(a.streamEvents) > 0 {
		for _, event := range a.streamEvents {
			callback(event)
		}
		return nil
	}

	if callback != nil {
		callback(common.ChatStreamData{Status: common.ChatStreamStatusFinished, Data: "fixed text"})
	}
	return nil
}
func (a *aiCommandTestAPI) GetUpdatableResult(ctx context.Context, resultId string) *plugin.UpdatableResult {
	title := "AI command"
	subTitle := ""
	preview := plugin.WoxPreview{}
	actions := []plugin.QueryResultAction{}
	tails := []plugin.QueryResultTail{}
	icon := common.WoxImage{}
	return &plugin.UpdatableResult{
		Id:       resultId,
		Title:    &title,
		SubTitle: &subTitle,
		Icon:     &icon,
		Preview:  &preview,
		Actions:  &actions,
		Tails:    &tails,
	}
}
func (a *aiCommandTestAPI) UpdateResult(ctx context.Context, result plugin.UpdatableResult) bool {
	return true
}
func (a *aiCommandTestAPI) PushResults(ctx context.Context, query plugin.Query, results []plugin.QueryResult) bool {
	return false
}
func (a *aiCommandTestAPI) IsVisible(ctx context.Context) bool { return false }
func (a *aiCommandTestAPI) RefreshQuery(ctx context.Context, param plugin.RefreshQueryParam) {
}
func (a *aiCommandTestAPI) RefreshGlance(ctx context.Context, ids []string) {
}
func (a *aiCommandTestAPI) Copy(ctx context.Context, params plugin.CopyParams) {
}
func (a *aiCommandTestAPI) Screenshot(ctx context.Context, option plugin.ScreenshotOption) plugin.ScreenshotResult {
	return plugin.ScreenshotResult{}
}

func aiCommandTestCommand(defaultAction string) map[string]any {
	command := map[string]any{
		"name":    "Grammar",
		"command": "grammar",
		"model":   `{"Name":"gpt-test","Provider":"openai"}`,
		"prompt":  "Fix grammar: %s",
		"vision":  false,
	}
	if defaultAction != "" {
		command["defaultAction"] = defaultAction
	}
	return command
}

func aiCommandVisionTestCommand(defaultAction string) map[string]any {
	command := aiCommandTestCommand(defaultAction)
	command["vision"] = true
	return command
}

func findAICommandAction(t *testing.T, actions []plugin.QueryResultAction, name string) plugin.QueryResultAction {
	t.Helper()

	for _, action := range actions {
		if action.Name == name {
			return action
		}
	}
	t.Fatalf("action %q not found in %#v", name, actions)
	return plugin.QueryResultAction{}
}

func TestAICommandQueryCommandIsLazyAndDefaultsToRun(t *testing.T) {
	api := newAICommandTestAPI(t, []map[string]any{aiCommandTestCommand("")})
	p := &Plugin{api: api}

	results := p.queryCommand(context.Background(), plugin.Query{Command: "grammar", Search: "this are bad"})
	require.Len(t, results, 1)

	select {
	case <-api.streamDone:
		t.Fatal("querying an AI command should not start the AI stream before an action runs")
	case <-time.After(150 * time.Millisecond):
	}
	require.Equal(t, 0, api.streamCallCount())

	runAction := findAICommandAction(t, results[0].Actions, "i18n:plugin_ai_command_run")
	runAndPasteAction := findAICommandAction(t, results[0].Actions, "i18n:plugin_ai_command_run_and_paste")
	require.True(t, runAction.IsDefault)
	require.False(t, runAndPasteAction.IsDefault)

	runAction.Action(context.Background(), plugin.ActionContext{ResultId: results[0].Id})
	select {
	case <-api.streamDone:
	case <-time.After(time.Second):
		t.Fatal("Run action did not start the AI stream")
	}
}

func TestAICommandDefaultActionCanRunAndPaste(t *testing.T) {
	api := newAICommandTestAPI(t, []map[string]any{aiCommandTestCommand("run_and_paste")})
	p := &Plugin{api: api}

	results := p.queryCommand(context.Background(), plugin.Query{Command: "grammar", Search: "this are bad"})
	require.Len(t, results, 1)

	runAction := findAICommandAction(t, results[0].Actions, "i18n:plugin_ai_command_run")
	runAndPasteAction := findAICommandAction(t, results[0].Actions, "i18n:plugin_ai_command_run_and_paste")
	require.False(t, runAction.IsDefault)
	require.True(t, runAndPasteAction.IsDefault)
	require.Equal(t, 0, api.streamCallCount())
}

func TestAICommandRequestsActiveWindowQueryEnv(t *testing.T) {
	p := &Plugin{}

	metadata := p.GetMetadata()
	params, err := metadata.GetFeatureParamsForQueryEnv()
	require.NoError(t, err)
	require.True(t, params.RequireActiveWindowName)
	require.True(t, params.RequireActiveWindowPid)
	require.True(t, params.RequireActiveWindowIcon)
}

func TestAICommandRunAndPasteNotifiesStreamErrors(t *testing.T) {
	previousShowOverlay := aiCommandShowOverlay
	previousCloseOverlay := aiCommandCloseOverlay
	aiCommandShowOverlay = func(opts overlay.OverlayOptions) {}
	aiCommandCloseOverlay = func(name string) {}
	t.Cleanup(func() {
		aiCommandShowOverlay = previousShowOverlay
		aiCommandCloseOverlay = previousCloseOverlay
	})

	api := newAICommandTestAPI(t, []map[string]any{aiCommandTestCommand("run_and_paste")})
	api.streamEvents = []common.ChatStreamData{
		{Status: common.ChatStreamStatusStreaming, Data: "partial answer"},
		{Status: common.ChatStreamStatusError, Data: "model failed"},
	}
	p := &Plugin{api: api}

	results := p.queryCommand(context.Background(), plugin.Query{Command: "grammar", Search: "this are bad"})
	require.Len(t, results, 1)

	runAndPasteAction := findAICommandAction(t, results[0].Actions, "i18n:plugin_ai_command_run_and_paste")
	runAndPasteAction.Action(context.Background(), plugin.ActionContext{ResultId: results[0].Id})

	require.Equal(t, "AI command action failed: model failed", api.waitForNotification(t))
}

func TestAICommandSelectionUsesExplicitActionsAndSkipsPasteForVision(t *testing.T) {
	t.Run("text selection can default to run and paste", func(t *testing.T) {
		api := newAICommandTestAPI(t, []map[string]any{aiCommandTestCommand("run_and_paste")})
		p := &Plugin{api: api}

		results := p.querySelection(context.Background(), plugin.Query{
			Type: plugin.QueryTypeSelection,
			Selection: selection.Selection{
				Type: selection.SelectionTypeText,
				Text: "this are bad",
			},
		})
		require.Len(t, results, 1)

		runAction := findAICommandAction(t, results[0].Actions, "i18n:plugin_ai_command_run")
		runAndPasteAction := findAICommandAction(t, results[0].Actions, "i18n:plugin_ai_command_run_and_paste")
		require.False(t, runAction.IsDefault)
		require.True(t, runAndPasteAction.IsDefault)
		require.Equal(t, 0, api.streamCallCount())
	})

	t.Run("vision selection only exposes run", func(t *testing.T) {
		api := newAICommandTestAPI(t, []map[string]any{aiCommandVisionTestCommand("run_and_paste")})
		p := &Plugin{api: api}

		results := p.querySelection(context.Background(), plugin.Query{
			Type: plugin.QueryTypeSelection,
			Selection: selection.Selection{
				Type:      selection.SelectionTypeFile,
				FilePaths: []string{"/tmp/image.png"},
			},
		})
		require.Len(t, results, 1)

		runAction := findAICommandAction(t, results[0].Actions, "i18n:plugin_ai_command_run")
		require.True(t, runAction.IsDefault)
		for _, action := range results[0].Actions {
			require.NotEqual(t, "i18n:plugin_ai_command_run_and_paste", action.Name)
		}
		require.Equal(t, 0, api.streamCallCount())
	})
}
