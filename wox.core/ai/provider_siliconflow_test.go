package ai

import (
	"context"
	"testing"
	"wox/common"
	"wox/setting"
)

func TestSiliconFlowRejectsEmbeddingModelsForChat(t *testing.T) {
	if reason := siliconFlowNonChatModelReason("BAAI/bge-large-zh-v1.5"); reason == "" {
		t.Fatal("embedding model must be rejected before chat/completions")
	}
	if reason := siliconFlowNonChatModelReason("Qwen/Qwen2.5-7B-Instruct"); reason != "" {
		t.Fatalf("chat model was rejected: %s", reason)
	}
}

func TestSiliconFlowListsChatModelsOnly(t *testing.T) {
	provider := NewSiliconFlowProvider(context.Background(), setting.AIProvider{Name: "siliconflow"}).(*SiliconFlowProvider)
	if provider.options.ModelsQueryParams["sub_type"] != "chat" {
		t.Fatalf("ModelsQueryParams = %v, want sub_type=chat", provider.options.ModelsQueryParams)
	}
}

func TestSiliconFlowThinkingModeOnlySetsEnableThinkingWhenChosen(t *testing.T) {
	provider := NewSiliconFlowProvider(context.Background(), setting.AIProvider{Name: "siliconflow"}).(*SiliconFlowProvider)
	model := common.Model{Name: "Qwen/Qwen3-8B", Provider: "siliconflow"}

	if extra := provider.getChatRequestExtraBody(context.Background(), model, nil, common.ChatOptions{}); len(extra) != 0 {
		t.Fatalf("provider default extra body = %v, want none", extra)
	}
	if extra := provider.getChatRequestExtraBody(context.Background(), model, nil, common.ChatOptions{ThinkingMode: common.ChatThinkingModeThinking}); extra["enable_thinking"] != true {
		t.Fatalf("thinking mode extra body = %v, want enable_thinking=true", extra)
	}
	if extra := provider.getChatRequestExtraBody(context.Background(), model, nil, common.ChatOptions{ThinkingMode: common.ChatThinkingModeNonThinking}); extra["enable_thinking"] != false {
		t.Fatalf("non-thinking mode extra body = %v, want enable_thinking=false", extra)
	}
}
