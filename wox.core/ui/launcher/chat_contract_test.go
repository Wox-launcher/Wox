package launcher

import (
	"encoding/json"
	"reflect"
	"testing"
	woxui "wox/ui/runtime"

	"wox/ai"
	"wox/common"
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
