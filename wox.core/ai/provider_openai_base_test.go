package ai

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"wox/common"
	"wox/setting"
	"wox/util"
)

func TestDeepSeekConversationReplayPreservesReasoning(t *testing.T) {
	provider := NewOpenAIBaseProvider(setting.AIProvider{Name: "deepseek", Host: "https://api.deepseek.com"})
	messages, convertErr := provider.convertConversations([]common.Conversation{
		{Role: common.ConversationRoleUser, Text: "weather"},
		{Role: common.ConversationRoleAssistant, Text: "Let me check.", Reasoning: "I need the weather tool."},
		{Role: common.ConversationRoleTool, ToolCallInfo: common.ToolCallInfo{Id: "call-1", Name: "weather", Delta: `{}`, Response: "sunny"}},
		{Role: common.ConversationRoleAssistant, Text: "It is sunny.", Reasoning: "The tool returned sunny."},
	})

	if convertErr != nil {
		t.Fatal(convertErr)
	}
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

func TestChatImageAttachmentsReachProviderAsImageContent(t *testing.T) {
	directory := t.TempDir()
	previous := util.GetLocation().GetUserDataDirectory()
	util.GetLocation().UpdateUserDataDirectory(directory)
	t.Cleanup(func() { util.GetLocation().UpdateUserDataDirectory(previous) })
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "image.png")
	if err := os.WriteFile(path, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	attachment, err := common.ImportChatAttachment(path)
	if err != nil {
		t.Fatal(err)
	}
	provider := &OpenAIBaseProvider{}
	messages, err := provider.convertConversations([]common.Conversation{{Role: common.ConversationRoleUser, Text: "Describe", Attachments: []common.AIChatAttachment{attachment}}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(messages)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"type":"image_url"`) || !strings.Contains(string(data), "data:image/png;base64,") || strings.Contains(string(data), "chat-attachment:") {
		t.Fatalf("provider payload = %s", data)
	}
	legacy, err := provider.convertConversations([]common.Conversation{{Role: common.ConversationRoleUser, Text: "Describe", Images: []common.WoxImage{common.NewWoxImageAbsolutePath(path)}}})
	if err != nil {
		t.Fatal(err)
	}
	legacyJSON, _ := json.Marshal(legacy)
	if !bytes.Equal(data, legacyJSON) {
		t.Fatal("legacy Images and attachments should use the same provider content")
	}
	if err := os.Remove(common.ChatAttachmentPath(attachment)); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.convertConversations([]common.Conversation{{Role: common.ConversationRoleUser, Attachments: []common.AIChatAttachment{attachment}}}); err == nil {
		t.Fatal("missing image must fail instead of silently sending only text")
	}
}
