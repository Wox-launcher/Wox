package launcher

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"wox/ai"
	"wox/common"
	"wox/ui/contract"
	woxui "wox/ui/runtime"
)

func TestChatDataContractRoundTrip(t *testing.T) {
	source := chatData{
		ID: "chat-1", Title: "Architecture", Model: aiModel{Name: "gpt", Provider: "openai", ProviderAlias: "work"},
		Conversations: []chatConversation{{
			ID: "message-1", Role: "user", Text: "hello",
			Images:    []woxImage{{ImageType: "emoji", ImageData: "👋"}},
			SkillRefs: []chatSkillRef{{ID: "skill-1", Name: "Review", Path: "/tmp/review", Source: "local"}},
		}},
	}
	contractChat, err := chatDataToContract(source)
	if err != nil {
		t.Fatalf("chatDataToContract returned error: %v", err)
	}
	if contractChat.Id != "chat-1" || contractChat.Model != (common.Model{Name: "gpt", Provider: "openai", ProviderAlias: "work"}) {
		t.Fatalf("contract chat = %+v", contractChat)
	}
	roundTrip, err := chatDataFromContract(contractChat)
	if err != nil {
		t.Fatalf("chatDataFromContract returned error: %v", err)
	}
	if roundTrip.ID != source.ID || len(roundTrip.Conversations) != 1 || roundTrip.Conversations[0].Text != "hello" || len(roundTrip.Conversations[0].Images) != 1 {
		t.Fatalf("round trip chat = %+v", roundTrip)
	}
}

func TestChatRenderItemsCollapseCompletedReasoningRound(t *testing.T) {
	conversations := []chatConversation{
		{ID: "user-1", Role: "user", Text: "hello", Timestamp: 1_000},
		{ID: "assistant-1", Role: "assistant", Text: "answer", Reasoning: "process", Timestamp: 2_400},
	}

	collapsed := chatRenderItems(conversations, false, nil)
	if len(collapsed) != 3 || collapsed[1].kind != "round" || collapsed[1].roundExpanded || !collapsed[2].hideReasoning || !collapsed[2].showMeta {
		t.Fatalf("collapsed render items = %+v", collapsed)
	}
	expanded := chatRenderItems(conversations, false, map[string]bool{collapsed[1].roundID: true})
	if len(expanded) != 4 || !expanded[1].roundExpanded || expanded[2].conversation.Reasoning != "process" || expanded[2].conversation.Text != "" {
		t.Fatalf("expanded render items = %+v", expanded)
	}
	streaming := chatRenderItems(conversations, true, nil)
	if len(streaming) != 2 || streaming[1].kind == "round" || streaming[1].hideReasoning {
		t.Fatalf("streaming render items = %+v", streaming)
	}
	if duration := formatChatRoundDuration(1_000, 62_400); duration != "1m 1s" {
		t.Fatalf("duration = %q", duration)
	}
}

func TestChatRenderItemsGroupsConsecutiveToolCalls(t *testing.T) {
	tools := []chatConversation{
		{ID: "tool-1", Role: "tool", ToolCallInfo: chatToolCallInfo{Name: "web_search", Status: "succeeded"}},
		{ID: "tool-2", Role: "tool", ToolCallInfo: chatToolCallInfo{Name: "web_fetch", Status: "running"}},
	}

	items := chatRenderItems(tools, true, nil)
	if len(items) != 1 || items[0].kind != "tool-activity" || len(items[0].tools) != 2 || items[0].roundExpanded {
		t.Fatalf("tool activity = %+v", items)
	}
	expanded := chatRenderItems(tools, true, map[string]bool{items[0].roundID: true})
	if !expanded[0].roundExpanded {
		t.Fatal("tool activity did not retain its disclosure state")
	}
	if status := chatToolActivityStatus(tools); status != "running" {
		t.Fatalf("tool activity status = %q, want running", status)
	}
	tools[0].ToolCallInfo.Status = "failed"
	if status := chatToolActivityStatus(tools); status != "failed" {
		t.Fatalf("failed tool activity status = %q", status)
	}
}

func TestChatToolOriginLabel(t *testing.T) {
	if got := chatToolOriginLabel(chatToolCallInfo{Name: "search", Source: "mcp", Server: "ddg-search"}); got != "ddg-search/search" {
		t.Fatalf("mcp origin = %q", got)
	}
	if got := chatToolOriginLabel(chatToolCallInfo{Name: "web_search", Source: "builtin"}); got != "builtin/web_search" {
		t.Fatalf("builtin origin = %q", got)
	}
	if got := chatToolOriginLabel(chatToolCallInfo{Name: "search"}); got != "search" {
		t.Fatalf("unknown origin = %q", got)
	}
}

func TestChatToolCallFromContractFillsOriginFromRegistry(t *testing.T) {
	ai.GetToolRegistry().Register(common.Tool{
		Name:         "origin_contract_search",
		Source:       common.ToolSourceMCP,
		ServerConfig: &common.AIChatMCPServerConfig{Name: "ddg-search"},
	})
	t.Cleanup(func() {
		ai.GetToolRegistry().Unregister("origin_contract_search")
	})

	got := chatToolCallFromContract(common.ToolCallInfo{Id: "call-1", Name: "origin_contract_search"})
	if got.Source != string(common.ToolSourceMCP) || got.Server != "ddg-search" {
		t.Fatalf("contract origin = %+v", got)
	}

	persisted := chatToolCallFromContract(common.ToolCallInfo{
		Id: "call-2", Name: "search", Source: common.ToolSourceBuiltin,
	})
	if persisted.Source != string(common.ToolSourceBuiltin) || persisted.Server != "" {
		t.Fatalf("persisted origin should not be overwritten = %+v", persisted)
	}
}

func TestFormatChatQuoteMessageKeepsReferenceAboveFollowUp(t *testing.T) {
	got := common.ChatMessageText("What does this mean?", []common.AIChatAttachment{{Kind: common.AIChatAttachmentQuote, Text: "line 1\nline 2"}})
	want := "Quoted reference:\n> line 1\n> line 2\n\nWhat does this mean?"
	if got != want {
		t.Fatalf("quote message = %q, want %q", got, want)
	}
}

func TestChatPreviewDataDecodesInitialAttachments(t *testing.T) {
	var data chatPreviewData
	if err := json.Unmarshal([]byte(`{"ActiveChat":{"Id":"chat-1"},"InitialAttachments":[{"ID":"quote-1","Kind":"quote","Text":"selected text"}]}`), &data); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if data.ActiveChat.ID != "chat-1" || len(data.InitialAttachments) != 1 || data.InitialAttachments[0].Text != "selected text" {
		t.Fatalf("preview data = %+v", data)
	}
}

func TestChatAttachmentsRoundTripAndRestoreForEditing(t *testing.T) {
	attachments := []common.AIChatAttachment{{ID: "quote-1", Kind: common.AIChatAttachmentQuote, Text: "literal {skill:Example}\nline 2"}, {ID: "quote-2", Kind: common.AIChatAttachmentQuote, Text: "second quote"}}
	source := chatData{ID: "chat", Conversations: []chatConversation{{ID: "message", Role: "user", Text: "Explain", Attachments: attachments}}}
	contract, err := chatDataToContract(source)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := chatDataFromContract(contract)
	if err != nil {
		t.Fatal(err)
	}
	for _, restored := range []chatData{decoded, fromCoreChatData(contract), cloneChatData(source)} {
		if !reflect.DeepEqual(restored.Conversations[0].Attachments, attachments) {
			t.Fatalf("lost attachments: %+v", restored)
		}
	}
	app := &App{chatPreview: &chatPreviewState{chat: decoded, editor: woxui.NewTextEditor("")}}
	app.editChatConversation("message")
	if app.chatPreview.editor.State().Text != "Explain" || !reflect.DeepEqual(app.chatPreview.attachments, attachments) {
		t.Fatal("editing failed to restore separate instructions and attachments")
	}
	if unresolvedChatSkillTag(app.chatPreview.editor.State().Text, nil) != "" {
		t.Fatal("quoted skill tag entered the editor")
	}
	app.dismissChatAttachment("quote-1")
	if len(app.chatPreview.attachments) != 1 || app.chatPreview.attachments[0].ID != "quote-2" {
		t.Fatal("dismiss should remove only the selected attachment")
	}
	if !reflect.DeepEqual(contract.Conversations[0].Attachments, attachments) {
		t.Fatal("draft mutation changed history")
	}
	if got := chatConversationClipboardText(source.Conversations[0]); got != common.ChatMessageText("Explain", attachments) {
		t.Fatalf("copy text = %q", got)
	}
}

func TestNewChatClearsDraftAttachments(t *testing.T) {
	app := &App{chatPreview: &chatPreviewState{chat: chatData{Model: aiModel{Name: "model"}}, editor: woxui.NewTextEditor("old question"), attachments: []common.AIChatAttachment{{ID: "old", Kind: common.AIChatAttachmentQuote, Text: "old reference"}}}}
	app.startNewChat()
	if len(app.chatPreview.attachments) != 0 || app.chatPreview.editor.State().Text != "" {
		t.Fatal("new chat retained the previous draft")
	}
}

func TestNewChatLeavesActiveResponseRunning(t *testing.T) {
	app := &App{chatPreview: &chatPreviewState{
		chat:    chatData{ID: "streaming", Model: aiModel{Name: "model"}, IsStreaming: true, Conversations: []chatConversation{{ID: "u", Role: "user", Text: "hi"}}},
		editor:  woxui.NewTextEditor("draft"),
		sending: true,
		chats:   []chatData{{ID: "streaming", Title: "streaming", IsStreaming: true, IsSummary: true}},
	}}
	app.startNewChat()
	if app.chatPreview.error != "" {
		t.Fatalf("new chat error = %q, want empty", app.chatPreview.error)
	}
	if app.chatPreview.chat.ID == "streaming" || app.chatPreview.sending || app.chatPreview.chat.IsStreaming {
		t.Fatal("new draft should not inherit the previous stream")
	}
	if len(app.chatPreview.chats) == 0 || app.chatPreview.chats[0].ID != "streaming" || !app.chatPreview.chats[0].IsStreaming {
		t.Fatal("previous conversation must keep streaming in history")
	}
}

func TestSelectChatHistoryLeavesActiveResponseRunning(t *testing.T) {
	other := common.AIChatData{Id: "other", Title: "Other", Conversations: []common.Conversation{{Id: "u", Role: common.ConversationRoleUser, Text: "old"}}}
	app := &App{
		lifecycleCtx: context.Background(),
		services:     chatHistoryTestServices{chat: other},
		chatPreview: &chatPreviewState{
			chat:    chatData{ID: "streaming", Model: aiModel{Name: "model"}, IsStreaming: true, Conversations: []chatConversation{{ID: "u", Role: "user", Text: "hi"}}},
			editor:  woxui.NewTextEditor(""),
			sending: true,
			chats: []chatData{
				{ID: "streaming", Title: "streaming", IsStreaming: true, IsSummary: true},
				{ID: "other", Title: "Other", IsSummary: true},
			},
		},
	}
	app.selectChatHistory("other")
	if app.chatPreview.error != "" {
		t.Fatalf("switch error = %q, want empty", app.chatPreview.error)
	}
	if app.chatPreview.chat.ID != "other" || app.chatPreview.sending {
		t.Fatal("switch should show the selected chat without stopping the previous stream")
	}
	if !app.chatPreview.chats[0].IsStreaming || app.chatPreview.chats[0].ID != "streaming" {
		t.Fatal("background stream summary was cleared")
	}
	if app.chatPreview.panel != "history" || app.chatPreview.panelSelected != 1 {
		t.Fatalf("history panel = %q selected %d, want stay open on the chosen row", app.chatPreview.panel, app.chatPreview.panelSelected)
	}
}

func TestSelectChatHistoryKeepsSidebarOpen(t *testing.T) {
	current := chatData{ID: "current", Title: "Current", Conversations: []chatConversation{{ID: "u", Role: "user", Text: "hi"}}}
	app := &App{chatPreview: &chatPreviewState{
		chat: current, editor: woxui.NewTextEditor(""), panel: "history", panelSelected: 0,
		chats: []chatData{current, {ID: "other", Title: "Other", IsSummary: true}},
	}}
	app.selectChatHistory("current")
	if app.chatPreview.panel != "history" || app.chatPreview.panelSelected != 0 {
		t.Fatalf("reselecting the current chat closed the sidebar: panel=%q selected=%d", app.chatPreview.panel, app.chatPreview.panelSelected)
	}

	app.startNewChat()
	if app.chatPreview.panel != "history" {
		t.Fatalf("new chat from the sidebar closed it: panel=%q", app.chatPreview.panel)
	}
}

func TestApplyChatResponseUpdatesBackgroundStreamOnly(t *testing.T) {
	app := &App{chatPreview: &chatPreviewState{
		chat:  chatData{ID: "current", Title: "current"},
		chats: []chatData{{ID: "background", Title: "bg", IsStreaming: true, IsSummary: true}},
	}}
	app.applyChatResponse(chatData{
		ID: "background", Title: "updated", IsStreaming: true,
		Conversations: []chatConversation{{ID: "a", Role: "assistant", Text: "hello"}},
	})
	if app.chatPreview.chat.ID != "current" || app.chatPreview.chat.Title != "current" {
		t.Fatal("background snapshot replaced the visible chat")
	}
	if app.chatPreview.chats[0].ID != "background" || app.chatPreview.chats[0].Title != "updated" || !app.chatPreview.chats[0].IsStreaming {
		t.Fatal("background stream summary was not mirrored")
	}
}

type chatHistoryTestServices struct {
	contract.Services
	chat common.AIChatData
}

func TestSelectedModelSurvivesStreamingSnapshotsUntilNextSend(t *testing.T) {
	oldModel := aiModel{Name: "old", Provider: "openai"}
	nextModel := aiModel{Name: "next", Provider: "openai"}
	aiSettings := newAISettingsController(CommonDeps{Translate: func(s string) string { return s }})
	aiSettings.SetModels([]aiModel{nextModel})
	persisted := make(chan common.Model, 1)
	app := &App{
		lifecycleCtx: context.Background(), aiSettings: aiSettings,
		services:    chatModelTestServices{persisted: persisted},
		chatPreview: &chatPreviewState{active: true, chat: chatData{ID: "chat", Model: oldModel, IsStreaming: true}},
	}
	app.selectChatModel(0)
	select {
	case <-persisted:
	case <-time.After(5 * time.Second):
		t.Fatal("selected model was not persisted")
	}
	for _, streaming := range []bool{true, false} {
		app.applyChatResponse(chatData{ID: "chat", Model: oldModel, IsStreaming: streaming})
		if app.chatPreview.chat.Model != nextModel {
			t.Fatal("a server snapshot replaced the model selected for the next send")
		}
	}
	_, _, request := beginChatRequestLocked(app.chatPreview)
	if request.Model != nextModel || app.chatPreview.nextModel != nil {
		t.Fatal("next request did not consume the selected model")
	}
}

type chatModelTestServices struct {
	contract.Services
	persisted chan common.Model
}

func (s chatModelTestServices) SetDefaultChatModel(_ context.Context, _ string, model common.Model) error {
	s.persisted <- model
	return nil
}

func (s chatHistoryTestServices) ChatByID(context.Context, string, string) (common.AIChatData, error) {
	return s.chat, nil
}
