package ai

import (
	"encoding/json"
	"testing"
	"wox/common"
	"wox/setting"
)

func TestDeepSeekConversationReplayPreservesReasoning(t *testing.T) {
	provider := NewOpenAIBaseProvider(setting.AIProvider{Name: "deepseek", Host: "https://api.deepseek.com"})
	messages := provider.convertConversations([]common.Conversation{
		{Role: common.ConversationRoleUser, Text: "weather"},
		{Role: common.ConversationRoleAssistant, Text: "Let me check.", Reasoning: "I need the weather tool."},
		{Role: common.ConversationRoleTool, ToolCallInfo: common.ToolCallInfo{Id: "call-1", Name: "weather", Delta: `{}`, Response: "sunny"}},
		{Role: common.ConversationRoleAssistant, Text: "It is sunny.", Reasoning: "The tool returned sunny."},
	})

	if len(messages) != 4 {
		t.Fatalf("converted messages = %d, want 4", len(messages))
	}

	for index, expectedReasoning := range map[int]string{1: "I need the weather tool.", 3: "The tool returned sunny."} {
		data, err := json.Marshal(messages[index])
		if err != nil {
			t.Fatal(err)
		}
		var message map[string]any
		if err := json.Unmarshal(data, &message); err != nil {
			t.Fatal(err)
		}
		if message["reasoning_content"] != expectedReasoning {
			t.Fatalf("message %d reasoning_content = %v, want %q", index, message["reasoning_content"], expectedReasoning)
		}
	}

	data, err := json.Marshal(messages[1])
	if err != nil {
		t.Fatal(err)
	}
	var toolCallMessage map[string]any
	if err := json.Unmarshal(data, &toolCallMessage); err != nil {
		t.Fatal(err)
	}
	if toolCallMessage["content"] != "Let me check." {
		t.Fatalf("tool-call content = %v, want %q", toolCallMessage["content"], "Let me check.")
	}
	if len(toolCallMessage["tool_calls"].([]any)) != 1 {
		t.Fatalf("tool_calls = %v, want one call", toolCallMessage["tool_calls"])
	}
}
