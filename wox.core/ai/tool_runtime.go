package ai

import (
	"html"
	"sort"
	"strings"

	"wox/common"
)

const (
	// ReadSkillToolName is the runtime tool used for progressive skill loading.
	ReadSkillToolName = "read_skill"
	// LoadToolsToolName is the runtime tool used to request executable tools for the next loop step.
	LoadToolsToolName = "load_tools"
	// AskUserToolName is the built-in tool for asking the user a question through the chat UI.
	AskUserToolName = "ask_user"
	// WebSearchToolName is the built-in search tool exposed to AI chat.
	WebSearchToolName = "web_search"
	// WebFetchToolName is the built-in page fetch tool exposed to AI chat.
	WebFetchToolName = "web_fetch"

	availableToolsPromptMaxChars = 12000
)

// IsRuntimeOnlyTool reports whether a tool is only used to manage chat context.
func IsRuntimeOnlyTool(name string) bool {
	return name == ReadSkillToolName || name == LoadToolsToolName
}

// FormatAvailableToolsPrompt returns a lightweight catalog of tools the model can load.
func FormatAvailableToolsPrompt(tools []common.Tool) string {
	visibleTools := make([]common.Tool, 0, len(tools))
	for _, tool := range tools {
		if strings.TrimSpace(tool.Name) == "" || IsRuntimeOnlyTool(tool.Name) {
			continue
		}
		visibleTools = append(visibleTools, tool)
	}
	if len(visibleTools) == 0 {
		return ""
	}

	sort.Slice(visibleTools, func(i, j int) bool {
		if visibleTools[i].Source != visibleTools[j].Source {
			return visibleTools[i].Source < visibleTools[j].Source
		}
		iServer := toolServerName(visibleTools[i])
		jServer := toolServerName(visibleTools[j])
		if iServer != jServer {
			return iServer < jServer
		}
		return visibleTools[i].Name < visibleTools[j].Name
	})

	var builder strings.Builder
	builder.WriteString("Tool catalog: builtin tools are already enabled and callable now. MCP and other catalog tools are discoverable but must be loaded with load_tools before they can be called. Use load_tools with exact names from the tool_catalog. Loaded tools become callable in the next model step.\n")
	builder.WriteString("<tool_catalog>\n")

	count := 0
	for _, tool := range visibleTools {
		entry := formatAvailableToolEntry(tool)
		if builder.Len()+len(entry)+len("</tool_catalog>") > availableToolsPromptMaxChars {
			builder.WriteString("  <truncated>true</truncated>\n")
			break
		}

		builder.WriteString(entry)
		count++
	}
	builder.WriteString("</tool_catalog>")

	if count == 0 {
		return ""
	}
	return builder.String()
}

// AppendRequestedTools adds tools requested through load_tools to the next model step.
func AppendRequestedTools(current []common.Tool, toolCalls []common.ToolCallInfo) []common.Tool {
	requested := extractRequestedToolNames(toolCalls)
	if len(requested) == 0 {
		return current
	}

	return appendToolsByName(current, requested)
}

// ParseLoadToolNames accepts the load_tools schema and returns exact requested names.
func ParseLoadToolNames(args map[string]any) []string {
	names := []string{}
	appendName := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		names = append(names, value)
	}

	if rawName, ok := args["name"].(string); ok {
		for _, value := range strings.Split(rawName, ",") {
			appendName(value)
		}
	}
	if rawNames, ok := args["names"].([]string); ok {
		for _, value := range rawNames {
			appendName(value)
		}
	}
	if rawNames, ok := args["names"].([]any); ok {
		for _, value := range rawNames {
			if name, ok := value.(string); ok {
				appendName(name)
			}
		}
	}

	seen := map[string]bool{}
	unique := make([]string, 0, len(names))
	for _, name := range names {
		if seen[name] {
			continue
		}
		seen[name] = true
		unique = append(unique, name)
	}
	return unique
}

func extractRequestedToolNames(toolCalls []common.ToolCallInfo) []string {
	names := []string{}
	for _, toolCall := range toolCalls {
		if toolCall.Name != LoadToolsToolName {
			continue
		}
		names = append(names, ParseLoadToolNames(toolCall.Arguments)...)
	}
	return names
}

func appendToolsByName(current []common.Tool, names []string) []common.Tool {
	if len(names) == 0 {
		return current
	}

	seen := map[string]bool{}
	next := make([]common.Tool, 0, len(current)+len(names))
	for _, tool := range current {
		if strings.TrimSpace(tool.Name) == "" {
			continue
		}
		seen[tool.Name] = true
		next = append(next, tool)
	}

	for _, name := range names {
		if seen[name] || IsRuntimeOnlyTool(name) {
			continue
		}
		tool, ok := GetToolRegistry().Get(name)
		if !ok {
			continue
		}
		seen[name] = true
		next = append(next, tool)
	}
	return next
}

func formatAvailableToolEntry(tool common.Tool) string {
	var builder strings.Builder
	builder.WriteString(`  <tool name="`)
	builder.WriteString(html.EscapeString(tool.Name))
	builder.WriteString(`" source="`)
	builder.WriteString(html.EscapeString(string(tool.Source)))
	builder.WriteString(`"`)
	if serverName := toolServerName(tool); serverName != "" {
		builder.WriteString(` server="`)
		builder.WriteString(html.EscapeString(serverName))
		builder.WriteString(`"`)
	}
	builder.WriteString(`>`)
	if description := compactDescription(tool.Description, 360); description != "" {
		builder.WriteString("\n    <description>")
		builder.WriteString(html.EscapeString(description))
		builder.WriteString("</description>\n  ")
	}
	builder.WriteString("</tool>\n")
	return builder.String()
}

func toolServerName(tool common.Tool) string {
	if tool.ServerConfig == nil {
		return ""
	}
	return tool.ServerConfig.Name
}

func compactDescription(value string, maxLen int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= maxLen {
		return value
	}
	return strings.TrimSpace(value[:maxLen]) + "..."
}
