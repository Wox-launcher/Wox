package ai

import (
	"sort"
	"strings"
	"sync"

	"wox/common"
)

const (
	// TestQueryToolName is the built-in diagnostic query tool.
	TestQueryToolName = "test_query"
	// ListScriptPluginsToolName is the built-in script-plugin listing tool.
	ListScriptPluginsToolName = "list_script_plugins"
)

var (
	disabledBuiltinMu    sync.RWMutex
	disabledBuiltinTools []string
)

// IsAlwaysOnBuiltinTool reports whether a built-in tool stays available even
// when it appears in the user disable list. These tools are chat runtime or
// Wox-owned capabilities rather than optional model actions.
func IsAlwaysOnBuiltinTool(name string) bool {
	switch name {
	case LoadToolsToolName, ReadSkillToolName, AskUserToolName, TestQueryToolName, ListScriptPluginsToolName:
		return true
	default:
		return false
	}
}

// SetDisabledBuiltinTools replaces the process-wide disable list used by tool
// loading. Always-on names are ignored so they cannot be turned off.
func SetDisabledBuiltinTools(names []string) {
	disabledBuiltinMu.Lock()
	defer disabledBuiltinMu.Unlock()
	disabledBuiltinTools = SanitizeDisabledBuiltinTools(names)
}

// DisabledBuiltinTools returns a copy of the current disable list.
func DisabledBuiltinTools() []string {
	disabledBuiltinMu.RLock()
	defer disabledBuiltinMu.RUnlock()
	return append([]string(nil), disabledBuiltinTools...)
}

// SanitizeDisabledBuiltinTools keeps unique configurable built-in tool names.
func SanitizeDisabledBuiltinTools(names []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || IsAlwaysOnBuiltinTool(name) || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

// IsBuiltinToolDisabled reports whether a configurable built-in tool is disabled.
func IsBuiltinToolDisabled(name string, disabled []string) bool {
	if name == "" || IsAlwaysOnBuiltinTool(name) {
		return false
	}
	for _, candidate := range disabled {
		if candidate == name {
			return true
		}
	}
	return false
}

// FilterDisabledBuiltinTools drops disabled built-in tools and keeps MCP tools.
func FilterDisabledBuiltinTools(tools []common.Tool, disabled []string) []common.Tool {
	if len(tools) == 0 {
		return tools
	}
	out := make([]common.Tool, 0, len(tools))
	for _, tool := range tools {
		if tool.Source == common.ToolSourceBuiltin && IsBuiltinToolDisabled(tool.Name, disabled) {
			continue
		}
		out = append(out, tool)
	}
	return out
}

// ConfigurableBuiltinTools returns built-in tools that the user can enable or disable.
func ConfigurableBuiltinTools() []common.Tool {
	tools := GetToolRegistry().ListBySource(common.ToolSourceBuiltin)
	out := make([]common.Tool, 0, len(tools))
	for _, tool := range tools {
		if IsAlwaysOnBuiltinTool(tool.Name) {
			continue
		}
		out = append(out, tool)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// ConfigurableBuiltinToolInfos returns name and description for the settings catalog.
func ConfigurableBuiltinToolInfos() []common.AIConfigurableBuiltinTool {
	tools := ConfigurableBuiltinTools()
	out := make([]common.AIConfigurableBuiltinTool, 0, len(tools))
	for _, tool := range tools {
		out = append(out, common.AIConfigurableBuiltinTool{Name: tool.Name, Description: BuiltinToolDescriptionKey(tool.Name)})
	}
	return out
}

// BuiltinToolDescriptionKey is the settings-page i18n key for a configurable built-in tool.
func BuiltinToolDescriptionKey(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return "i18n:ui_ai_builtin_tool_desc_" + name
}
