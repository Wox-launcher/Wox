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
	if len(provider.options.ModelsListOptions) != 1 {
		t.Fatalf("ModelsListOptions = %d, want chat sub_type filter", len(provider.options.ModelsListOptions))
	}
}

func TestSiliconFlowThinkingModeOnlySetsEnableThinkingWhenChosen(t *testing.T) {
	provider := NewSiliconFlowProvider(context.Background(), setting.AIProvider{Name: "siliconflow"}).(*SiliconFlowProvider)
	model := common.Model{Name: "Qwen/Qwen3-8B", Provider: "siliconflow"}

	if opts := provider.getChatRequestOptions(context.Background(), model, nil, common.ChatOptions{}); len(opts) != 0 {
		t.Fatalf("provider default options = %d, want none", len(opts))
	}
	if opts := provider.getChatRequestOptions(context.Background(), model, nil, common.ChatOptions{ThinkingMode: common.ChatThinkingModeThinking}); len(opts) != 1 {
		t.Fatalf("thinking mode options = %d, want 1", len(opts))
	}
	if opts := provider.getChatRequestOptions(context.Background(), model, nil, common.ChatOptions{ThinkingMode: common.ChatThinkingModeNonThinking}); len(opts) != 1 {
		t.Fatalf("non-thinking mode options = %d, want 1", len(opts))
	}
}
