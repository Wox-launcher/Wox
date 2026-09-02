package ai

import (
	"testing"

	"wox/common"
)

func TestIsAlwaysOnBuiltinTool(t *testing.T) {
	alwaysOn := []string{LoadToolsToolName, ReadSkillToolName, AskUserToolName, TestQueryToolName, ListScriptPluginsToolName}
	for _, name := range alwaysOn {
		if !IsAlwaysOnBuiltinTool(name) {
			t.Fatalf("%s should stay always on", name)
		}
	}
	for _, name := range []string{"read", "write", "edit", "bash", "web_search", "web_fetch"} {
		if IsAlwaysOnBuiltinTool(name) {
			t.Fatalf("%s should be user-toggleable", name)
		}
	}
}

func TestBuiltinToolDescriptionKey(t *testing.T) {
	if got := BuiltinToolDescriptionKey("read"); got != "i18n:ui_ai_builtin_tool_desc_read" {
		t.Fatalf("description key = %q", got)
	}
}

func TestSanitizeDisabledBuiltinToolsIgnoresAlwaysOnAndDuplicates(t *testing.T) {
	got := SanitizeDisabledBuiltinTools([]string{"read", "load_tools", "ask_user", "read", " bash ", ""})
	if len(got) != 2 || got[0] != "read" || got[1] != "bash" {
		t.Fatalf("sanitized = %#v, want [read bash]", got)
	}
}

func TestFilterDisabledBuiltinToolsKeepsAlwaysOnAndMCP(t *testing.T) {
	tools := []common.Tool{
		{Name: "read", Source: common.ToolSourceBuiltin},
		{Name: AskUserToolName, Source: common.ToolSourceBuiltin},
		{Name: LoadToolsToolName, Source: common.ToolSourceBuiltin},
		{Name: "web_search", Source: common.ToolSourceBuiltin},
		{Name: "custom_search", Source: common.ToolSourceMCP},
	}
	filtered := FilterDisabledBuiltinTools(tools, []string{"read", "web_search", AskUserToolName})
	names := make([]string, 0, len(filtered))
	for _, tool := range filtered {
		names = append(names, tool.Name)
	}
	if len(names) != 3 || names[0] != AskUserToolName || names[1] != LoadToolsToolName || names[2] != "custom_search" {
		t.Fatalf("filtered = %#v", names)
	}
}

func TestAppendRequestedToolsSkipsDisabledBuiltins(t *testing.T) {
	SetDisabledBuiltinTools([]string{"read"})
	t.Cleanup(func() { SetDisabledBuiltinTools(nil) })
	GetToolRegistry().Register(common.Tool{Name: "read", Source: common.ToolSourceBuiltin})
	GetToolRegistry().Register(common.Tool{Name: "web_fetch", Source: common.ToolSourceBuiltin})

	next := AppendRequestedTools(nil, []common.ToolCallInfo{{
		Name:      LoadToolsToolName,
		Arguments: map[string]any{"names": []string{"read", "web_fetch"}},
	}})
	if len(next) != 1 || next[0].Name != "web_fetch" {
		t.Fatalf("loaded tools = %#v, want only web_fetch", next)
	}
}
