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
