package ai

import (
	"testing"

	"wox/common"
)

func TestAnnotateToolCallUsesVisibleToolThenRegistry(t *testing.T) {
	GetToolRegistry().Register(common.Tool{
		Name:         "origin_test_search",
		Source:       common.ToolSourceMCP,
		ServerConfig: &common.AIChatMCPServerConfig{Name: "ddg-search"},
	})
	t.Cleanup(func() {
		GetToolRegistry().Unregister("origin_test_search")
	})

	fromRegistry := common.ToolCallInfo{Name: "origin_test_search"}
	AnnotateToolCall(&fromRegistry, nil)
	if fromRegistry.Source != common.ToolSourceMCP || fromRegistry.Server != "ddg-search" {
		t.Fatalf("registry origin = %+v", fromRegistry)
	}

	fromVisible := common.ToolCallInfo{Name: "origin_test_search"}
	AnnotateToolCall(&fromVisible, []common.Tool{{
		Name:   "origin_test_search",
		Source: common.ToolSourceBuiltin,
	}})
	if fromVisible.Source != common.ToolSourceBuiltin || fromVisible.Server != "" {
		t.Fatalf("visible origin = %+v", fromVisible)
	}
}

func TestAnnotateToolCallsFillsSlice(t *testing.T) {
	calls := []common.ToolCallInfo{
		{Name: "web_search"},
		{Name: ""},
	}
	AnnotateToolCalls(calls, []common.Tool{{
		Name:   "web_search",
		Source: common.ToolSourceBuiltin,
	}})
	if calls[0].Source != common.ToolSourceBuiltin {
		t.Fatalf("annotated source = %q", calls[0].Source)
	}
	if calls[1].Source != "" {
		t.Fatalf("empty name should stay unannotated, got %+v", calls[1])
	}
}
