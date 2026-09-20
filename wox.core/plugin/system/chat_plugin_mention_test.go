package system

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"wox/ai"
	"wox/common"
	"wox/plugin"
)

type chatPluginToolAPI struct {
	emptyChatAPI
	tools   []plugin.PluginToolListItem
	invoked plugin.InvokePluginToolOption
}

func (a *chatPluginToolAPI) ListPluginTools(context.Context, plugin.ListPluginToolsOption) plugin.ListPluginToolsResult {
	return plugin.ListPluginToolsResult{Tools: a.tools}
}

func (a *chatPluginToolAPI) InvokePluginTool(_ context.Context, option plugin.InvokePluginToolOption) plugin.InvokePluginToolResult {
	a.invoked = option
	if option.Name == "create_note" {
		return plugin.InvokePluginToolResult{Output: map[string]any{"noteId": "n1"}}
	}
	return plugin.InvokePluginToolResult{Error: &plugin.PluginToolError{Code: plugin.PluginToolErrorNotFound, Message: "missing"}}
}

func TestListMentionablePluginsGroupsByPlugin(t *testing.T) {
	api := &chatPluginToolAPI{tools: []plugin.PluginToolListItem{
		{PluginId: common.AIChatPluginID, PluginName: "AI Chat", Tool: plugin.PluginToolDescriptor{Name: "open_chat_with_attachments"}},
		{PluginId: "notes", PluginName: "Notes", Tool: plugin.PluginToolDescriptor{Name: "create_note"}},
		{PluginId: "notes", PluginName: "Notes", Tool: plugin.PluginToolDescriptor{Name: "open_note"}},
		{PluginId: "files", PluginName: "File Search", Tool: plugin.PluginToolDescriptor{Name: "search"}},
	}}
	mentions := (&AIChatPlugin{api: api}).ListMentionablePlugins(context.Background())
	if len(mentions) != 2 || mentions[0].Name != "File Search" || mentions[1].Name != "Notes" {
		t.Fatalf("mentions = %+v", mentions)
	}
}

func TestPluginToolsForMentionsAreImmediatelyCallable(t *testing.T) {
	api := &chatPluginToolAPI{tools: []plugin.PluginToolListItem{
		{PluginId: "notes", PluginName: "Notes", Tool: plugin.PluginToolDescriptor{
			Name:        "create_note",
			Description: "Create a note",
			InputSchema: map[string]any{"type": "object", "properties": map[string]any{"title": map[string]any{"type": "string"}}},
		}},
	}}
	chat := &AIChatPlugin{api: api}
	tools := chat.pluginToolsForMentions(context.Background(), []string{"notes"})
	if len(tools) != 1 || tools[0].Name != ai.PluginToolAIName("notes", "create_note") || tools[0].Source != common.ToolSourcePlugin {
		t.Fatalf("tools = %+v", tools)
	}
	result, err := tools[0].Callback(context.Background(), map[string]any{"title": "Roadmap"})
	if err != nil {
		t.Fatal(err)
	}
	if api.invoked.PluginId != "notes" || api.invoked.Name != "create_note" {
		t.Fatalf("invoked = %+v", api.invoked)
	}
	var output map[string]any
	if err := json.Unmarshal([]byte(result.Text), &output); err != nil || output["noteId"] != "n1" {
		t.Fatalf("result = %q", result.Text)
	}
}

func TestUnmentionedPluginToolsAreNotLoaded(t *testing.T) {
	api := &chatPluginToolAPI{tools: []plugin.PluginToolListItem{
		{PluginId: "notes", PluginName: "Notes", Tool: plugin.PluginToolDescriptor{Name: "create_note"}},
		{PluginId: "files", PluginName: "File Search", Tool: plugin.PluginToolDescriptor{Name: "search"}},
	}}
	tools := (&AIChatPlugin{api: api}).pluginToolsForMentions(context.Background(), []string{"notes"})
	if len(tools) != 1 || tools[0].Name != ai.PluginToolAIName("notes", "create_note") {
		t.Fatalf("tools = %+v", tools)
	}
}

func TestPluginToolIdentitySurvivesMentionCatalogChanges(t *testing.T) {
	schema := map[string]any{"type": "object", "properties": map[string]any{"count": map[string]any{"enum": []any{1, 2}}}}
	api := &chatPluginToolAPI{tools: []plugin.PluginToolListItem{
		{PluginId: "alpha", PluginName: "Notes", Tool: plugin.PluginToolDescriptor{Name: "create_note", InputSchema: schema}},
		{PluginId: "beta", PluginName: "Notes", Tool: plugin.PluginToolDescriptor{Name: "create_note", InputSchema: schema}},
	}}
	chat := &AIChatPlugin{api: api}
	before := chat.pluginToolsForMentions(t.Context(), []string{"beta"})[0]
	after := chat.pluginToolsForMentions(t.Context(), []string{"alpha", "beta"})
	if after[0].Name == before.Name || after[1].Name != before.Name {
		t.Fatalf("tool identity changed: before=%s after=%s,%s", before.Name, after[0].Name, after[1].Name)
	}
	if !reflect.DeepEqual(after[1].InputSchema, schema) {
		t.Fatalf("plugin schema changed: %+v", after[1].InputSchema)
	}
	if _, err := after[1].Callback(t.Context(), nil); err != nil || api.invoked.PluginId != "beta" {
		t.Fatalf("tool routed to %s: %v", api.invoked.PluginId, err)
	}
}

func TestWithMessageSkillReferencesExpandsPluginMentions(t *testing.T) {
	source := common.Conversation{
		Id:       "message",
		Role:     common.ConversationRoleUser,
		Text:     "{plugin:Notes} Create a note about lunch",
		Mentions: []common.AIMentionRef{{Kind: common.AIMentionKindPlugin, Id: "notes", Name: "Notes"}},
	}
	runtime := (&AIChatPlugin{api: &emptyChatAPI{}}).withMessageSkillReferences(context.Background(), source)
	if strings.Contains(runtime.Text, "{plugin:Notes}") {
		t.Fatalf("runtime kept plugin tag: %s", runtime.Text)
	}
	if !strings.Contains(runtime.Text, "Notes") || !strings.Contains(runtime.Text, "Create a note about lunch") {
		t.Fatalf("runtime text = %s", runtime.Text)
	}
	if source.Text != "{plugin:Notes} Create a note about lunch" {
		t.Fatal("runtime materialization mutated stored message")
	}
}

func TestMentionIDsUnionsConversationRefsByKind(t *testing.T) {
	ids := mentionIDs([]common.Conversation{
		{Role: common.ConversationRoleUser, Mentions: []common.AIMentionRef{{Kind: common.AIMentionKindPlugin, Id: "notes", Name: "Notes"}}},
		{Role: common.ConversationRoleUser, Mentions: []common.AIMentionRef{
			{Kind: common.AIMentionKindPlugin, Id: "notes", Name: "Notes"},
			{Kind: common.AIMentionKindPlugin, Id: "files", Name: "File Search"},
		}},
	}, common.AIMentionKindPlugin)
	if len(ids) != 2 || ids[0] != "notes" || ids[1] != "files" {
		t.Fatalf("ids = %v", ids)
	}
}
