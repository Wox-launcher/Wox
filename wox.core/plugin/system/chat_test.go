package system

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"wox/ai"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util/selection"
)

type chatTestAPI struct {
	emptyChatAPI
	translations map[string]string
	changed      common.PlainQuery
}

func (a *chatTestAPI) GetTranslation(ctx context.Context, key string) string {
	if value := a.translations[key]; value != "" {
		return value
	}
	return key
}

func (a *chatTestAPI) ChangeQuery(ctx context.Context, query common.PlainQuery) {
	a.changed = query
}

type emptyChatAPI struct{}

func (emptyChatAPI) ChangeQuery(context.Context, common.PlainQuery) {}
func (emptyChatAPI) HideApp(context.Context)                        {}
func (emptyChatAPI) ShowApp(context.Context)                        {}
func (emptyChatAPI) Notify(context.Context, string)                 {}
func (emptyChatAPI) PushAttention(context.Context, plugin.PushAttentionRequest) {
}
func (emptyChatAPI) Log(context.Context, plugin.LogLevel, string)  {}
func (emptyChatAPI) GetTranslation(context.Context, string) string { return "" }
func (emptyChatAPI) GetSetting(context.Context, string) string     { return "" }
func (emptyChatAPI) SaveSetting(context.Context, string, string, bool) {
}
func (emptyChatAPI) SetSetting(context.Context, plugin.SetSettingOption) plugin.SetSettingResult {
	return plugin.SetSettingResult{Success: true}
}
func (emptyChatAPI) OnSettingChanged(context.Context, func(context.Context, string, string)) {
}
func (emptyChatAPI) OnGetDynamicSetting(context.Context, func(context.Context, string) definition.PluginSettingDefinitionItem) {
}
func (emptyChatAPI) OnDeepLink(context.Context, func(context.Context, map[string]string)) {
}
func (emptyChatAPI) OnUnload(context.Context, func(context.Context)) {}
func (emptyChatAPI) OnMRURestore(context.Context, func(context.Context, plugin.MRUData) (*plugin.QueryResult, error)) {
}
func (emptyChatAPI) OnHandlePluginCommand(context.Context, plugin.PluginCommandHandler) {
}
func (emptyChatAPI) InvokePluginCommand(context.Context, plugin.PluginCommandRequest) (plugin.PluginCommandResult, error) {
	return plugin.PluginCommandResult{}, nil
}
func (emptyChatAPI) ShowToolbarMsg(context.Context, plugin.ToolbarMsg) {}
func (emptyChatAPI) ClearToolbarMsg(context.Context, string)           {}
func (emptyChatAPI) OnEnterPluginQuery(context.Context, func(context.Context)) {
}
func (emptyChatAPI) OnLeavePluginQuery(context.Context, func(context.Context)) {
}
func (emptyChatAPI) RegisterQueryCommands(context.Context, []plugin.MetadataCommand) {
}
func (emptyChatAPI) AIChatStream(context.Context, common.Model, []common.Conversation, common.ChatOptions, common.ChatStreamFunc) error {
	return nil
}
func (emptyChatAPI) GetUpdatableResult(context.Context, string) *plugin.UpdatableResult {
	return nil
}
func (emptyChatAPI) UpdateResult(context.Context, plugin.UpdatableResult) bool { return false }
func (emptyChatAPI) PushResults(context.Context, plugin.Query, []plugin.QueryResult) bool {
	return false
}
func (emptyChatAPI) IsVisible(context.Context) bool { return false }
func (emptyChatAPI) RefreshQuery(context.Context, plugin.RefreshQueryParam) {
}
func (emptyChatAPI) RefreshGlance(context.Context, []string) {}
func (emptyChatAPI) Copy(context.Context, plugin.CopyParams) {}
func (emptyChatAPI) Screenshot(context.Context, plugin.ScreenshotOption) plugin.ScreenshotResult {
	return plugin.ScreenshotResult{}
}
func (emptyChatAPI) GetCacheFolder(context.Context) string { return "" }

func TestAIChatPluginDeclaresSelectionQuery(t *testing.T) {
	metadata := (&AIChatPlugin{}).GetMetadata()
	if !metadata.IsSupportFeature(plugin.MetadataFeatureQuerySelection) {
		t.Fatal("AI Chat should declare querySelection so the selection hotkey can reach it")
	}
}

func TestAIChatQuerySelectionQuotesText(t *testing.T) {
	api := &chatTestAPI{translations: map[string]string{
		"plugin_ai_chat_selection_quote":            "Ask in AI Chat",
		"plugin_ai_chat_selection_characters_value": "%d chars",
	}}
	chatPlugin := &AIChatPlugin{api: api}

	response := chatPlugin.Query(context.Background(), plugin.Query{
		Type: plugin.QueryTypeSelection,
		Selection: selection.Selection{
			Type: selection.SelectionTypeText,
			Text: "  selected line  ",
		},
	})
	if response.Layout.ChatMode {
		t.Fatal("selection results must stay in the mixed selection list")
	}
	if len(response.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(response.Results))
	}
	result := response.Results[0]
	if result.Title != "Ask in AI Chat" || result.SubTitle != "" || result.Preview.PreviewType != plugin.WoxPreviewTypeText {
		t.Fatalf("selection result = %+v", result)
	}
	if len(result.Actions) != 1 || result.Actions[0].Action == nil {
		t.Fatal("selection result is missing the quote action")
	}

	result.Actions[0].Action(context.Background(), plugin.ActionContext{})
	if api.changed.QueryType != plugin.QueryTypeInput || api.changed.QueryText != "chat " {
		t.Fatalf("change query = %+v", api.changed)
	}
	if !strings.Contains(api.changed.ContextData[aiChatAttachmentsContextKey], "  selected line  ") {
		t.Fatalf("quote context = %#v", api.changed.ContextData)
	}
}

func TestAIChatQuerySelectionSkipsEmpty(t *testing.T) {
	chatPlugin := &AIChatPlugin{api: &chatTestAPI{}}
	empty := chatPlugin.Query(context.Background(), plugin.Query{
		Type:      plugin.QueryTypeSelection,
		Selection: selection.Selection{Type: selection.SelectionTypeText, Text: "   "},
	})
	if len(empty.Results) != 0 {
		t.Fatalf("empty text results = %d", len(empty.Results))
	}

	files := chatPlugin.Query(context.Background(), plugin.Query{
		Type:      plugin.QueryTypeSelection,
		Selection: selection.Selection{Type: selection.SelectionTypeFile, FilePaths: nil},
	})
	if len(files.Results) != 0 {
		t.Fatalf("file selection results = %d", len(files.Results))
	}
}

func TestAIChatQueryInputEmbedsInitialAttachments(t *testing.T) {
	chatPlugin := &AIChatPlugin{api: &chatTestAPI{}}
	response := chatPlugin.Query(context.Background(), plugin.Query{
		Type: plugin.QueryTypeInput,
		ContextData: common.ContextData{
			aiChatAttachmentsContextKey: `[{"ID":"quote","Kind":"quote","Text":"selected text"}]`,
		},
	})
	if !response.Layout.ChatMode || len(response.Results) != 1 {
		t.Fatalf("input response = %+v", response)
	}

	var preview common.AIChatPreviewData
	if err := json.Unmarshal([]byte(response.Results[0].Preview.PreviewData), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if len(preview.InitialAttachments) != 1 || preview.InitialAttachments[0].Text != "selected text" {
		t.Fatalf("initial attachments = %+v", preview.InitialAttachments)
	}
}

func TestChatAttachmentsPreserveQuotedSkillTags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(path, []byte("Review the user's text carefully."), 0600); err != nil {
		t.Fatal(err)
	}
	skill := common.Skill{Id: "attachment-test-skill", Name: "Review", ManifestPath: path, Enabled: true}
	ai.GetSkillRegistry().Register(skill)
	t.Cleanup(func() { ai.GetSkillRegistry().Unregister(skill.Id) })
	source := common.Conversation{Id: "message", Role: common.ConversationRoleUser, Text: "{skill:Review} Explain this",
		SkillRefs:   []common.AISkillRef{ai.SkillRefFromSkill(skill)},
		Attachments: []common.AIChatAttachment{{ID: "quote", Kind: common.AIChatAttachmentQuote, Text: "  literal {skill:Example}\nsecond line"}},
	}
	plugin := &AIChatPlugin{api: &chatTestAPI{}}
	runtime := withMessageAttachments(plugin.withMessageSkillReferences(context.Background(), source))
	if !strings.Contains(runtime.Text, ">   literal {skill:Example}\n> second line") || strings.Contains(runtime.Text, "{skill:Review}") || !strings.Contains(runtime.Text, "Review the user's text carefully.") {
		t.Fatalf("runtime text = %s", runtime.Text)
	}
	if len(runtime.Attachments) != 0 || source.Text != "{skill:Review} Explain this" || len(source.Attachments) != 1 {
		t.Fatal("runtime materialization mutated stored message")
	}
	summary := formatConversationsForCompactionSummary([]common.Conversation{source})
	if !strings.Contains(summary, "literal {skill:Example}") {
		t.Fatalf("summary lost reference: %s", summary)
	}
	plain := source
	plain.Attachments = nil
	if estimateConversationTokens([]common.Conversation{source}) <= estimateConversationTokens([]common.Conversation{plain}) {
		t.Fatal("attachment tokens were not counted")
	}
	clone := cloneAIConversation(source)
	clone.Attachments[0].Text = "changed"
	if source.Attachments[0].Text == "changed" {
		t.Fatal("snapshot shares attachment storage")
	}
}

func TestAIChatSelectionForMultipleFilesUsesPathsOnly(t *testing.T) {
	directory := t.TempDir()
	paths := []string{filepath.Join(directory, "one.pdf"), filepath.Join(directory, "two.bin")}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("secret file contents"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	api := &chatTestAPI{}
	chatPlugin := &AIChatPlugin{api: api}
	response := chatPlugin.Query(context.Background(), plugin.Query{Type: plugin.QueryTypeSelection, Selection: selection.Selection{Type: selection.SelectionTypeFile, FilePaths: paths}})
	if len(response.Results) != 1 || response.Layout.ChatMode {
		t.Fatalf("selection response = %+v", response)
	}
	response.Results[0].Actions[0].Action(context.Background(), plugin.ActionContext{})
	var attachments []common.AIChatAttachment
	if err := json.Unmarshal([]byte(api.changed.ContextData[aiChatAttachmentsContextKey]), &attachments); err != nil {
		t.Fatal(err)
	}
	if len(attachments) != 2 {
		t.Fatalf("attachments = %+v", attachments)
	}
	for i, attachment := range attachments {
		if attachment.Kind != common.AIChatAttachmentFile || attachment.URL != paths[i] || attachment.Text != "" {
			t.Fatalf("file reference = %+v", attachment)
		}
	}
	message := withMessageAttachments(common.Conversation{Role: common.ConversationRoleUser, Text: "Compare", Attachments: attachments})
	if !strings.Contains(message.Text, paths[0]) || !strings.Contains(message.Text, paths[1]) || strings.Contains(message.Text, "secret file contents") {
		t.Fatalf("runtime message = %+v", message)
	}
}
