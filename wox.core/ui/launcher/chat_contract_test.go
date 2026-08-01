package launcher

import (
	"testing"

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
