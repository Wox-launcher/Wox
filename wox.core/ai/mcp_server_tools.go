package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"wox/util"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// parseToolArguments parses the raw JSON arguments from a tool request
func parseToolArguments(req *mcp.CallToolRequest) map[string]any {
	var args map[string]any
	if req.Params.Arguments != nil {
		_ = json.Unmarshal(req.Params.Arguments, &args)
	}
	if args == nil {
		args = make(map[string]any)
	}
	return args
}

func handleGetPluginSDKDocs(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseToolArguments(req)
	runtime, _ := args["runtime"].(string)

	var docs string
	switch runtime {
	case "nodejs":
		docs = getNodeJSSDKDocs()
	case "python":
		docs = getPythonSDKDocs()
	case "script":
		docs = getScriptPluginDocs()
	default:
		docs = "Please specify a runtime: nodejs, python, or script"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: docs},
		},
	}, nil
}

func handleGetPluginJsonSchema(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	schema := getPluginJsonSchema()
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: schema},
		},
	}, nil
}

func handleGeneratePluginScaffold(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := parseToolArguments(req)
	runtime, _ := args["runtime"].(string)
	name, _ := args["name"].(string)
	triggerKeywordsRaw, _ := args["trigger_keywords"].([]any)
	description, _ := args["description"].(string)

	if description == "" {
		description = fmt.Sprintf("A %s plugin for Wox", name)
	}

	var triggerKeywords []string
	for _, kw := range triggerKeywordsRaw {
		if s, ok := kw.(string); ok {
			triggerKeywords = append(triggerKeywords, s)
		}
	}

	var scaffold string
	switch runtime {
	case "nodejs":
		scaffold = generateNodeJSScaffold(name, description, triggerKeywords)
	case "python":
		scaffold = generatePythonScaffold(name, description, triggerKeywords)
	case "script":
		scaffold = generateScriptScaffold(name, description, triggerKeywords)
	default:
		scaffold = "Please specify a runtime: nodejs, python, or script"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: scaffold},
		},
	}, nil
}

func handleGetWoxDirectories(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	location := util.GetLocation()
	dirs := map[string]string{
		"wox_data_directory":            location.GetWoxDataDirectory(),
		"user_data_directory":           location.GetUserDataDirectory(),
		"plugins_directory":             location.GetPluginDirectory(),
		"user_script_plugins_directory": location.GetUserScriptPluginsDirectory(),
		"log_directory":                 location.GetLogDirectory(),
	}

	jsonBytes, _ := json.MarshalIndent(dirs, "", "  ")
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonBytes)},
		},
	}, nil
}

func handlePluginOverview(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	overview := getPluginOverview()
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: overview},
		},
	}, nil
}

func getPluginOverview() string {
	return mustRenderMcpPrompt(
		"plugin_overview.md",
		nil,
		"# Wox Plugin Overview\n\n(Template unavailable)",
	)
}

func getPluginJsonSchema() string {
	exampleID := uuid.New().String()
	return mustRenderMcpPrompt(
		"plugin_json_schema.md",
		map[string]any{"ExampleID": exampleID},
		"# plugin.json Schema\n\n(Template unavailable)",
	)
}

func getNodeJSSDKDocs() string {
	return mustRenderMcpPrompt(
		"sdk_nodejs.md",
		nil,
		"# Wox Node.js Plugin SDK\n\n(Template unavailable)",
	)
}

func getPythonSDKDocs() string {
	return mustRenderMcpPrompt(
		"sdk_python.md",
		nil,
		"# Wox Python Plugin SDK\n\n(Template unavailable)",
	)
}

func getScriptPluginDocs() string {
	return generateScriptScaffold("demo", "A demo script plugin", []string{"demo"})
}

func generateNodeJSScaffold(name, description string, triggerKeywords []string) string {
	pluginID := uuid.New().String()
	keywords, _ := json.Marshal(triggerKeywords)
	return mustRenderMcpPrompt(
		"scaffold_nodejs.md",
		map[string]any{
			"Name":                name,
			"Description":         description,
			"PluginID":            pluginID,
			"TriggerKeywordsJSON": string(keywords),
			"PascalName":          toPascalCase(name),
			"KebabName":           toKebabCase(name),
		},
		"# Node.js Plugin Scaffold\n\n(Template unavailable)",
	)
}

func generatePythonScaffold(name, description string, triggerKeywords []string) string {
	pluginID := uuid.New().String()
	keywords, _ := json.Marshal(triggerKeywords)
	return mustRenderMcpPrompt(
		"scaffold_python.md",
		map[string]any{
			"Name":                name,
			"Description":         description,
			"PluginID":            pluginID,
			"TriggerKeywordsJSON": string(keywords),
			"PascalName":          toPascalCase(name),
			"KebabName":           toKebabCase(name),
		},
		"# Python Plugin Scaffold\n\n(Template unavailable)",
	)
}

func generateScriptScaffold(name, description string, triggerKeywords []string) string {
	pluginID := uuid.New().String()
	keywords, _ := json.Marshal(triggerKeywords)

	pythonScript := mustRenderMcpTemplateFromScriptTemplates(
		"template.py",
		map[string]any{
			"Name":                name,
			"Description":         description,
			"PluginID":            pluginID,
			"TriggerKeywordsJSON": string(keywords),
		},
		"",
	)

	nodeScript := mustRenderMcpTemplateFromScriptTemplates(
		"template.js",
		map[string]any{
			"Name":                name,
			"Description":         description,
			"PluginID":            pluginID,
			"TriggerKeywordsJSON": string(keywords),
		},
		"",
	)

	bashScript := mustRenderMcpTemplateFromScriptTemplates(
		"template.sh",
		map[string]any{
			"Name":                name,
			"Description":         description,
			"PluginID":            pluginID,
			"TriggerKeywordsJSON": string(keywords),
		},
		"",
	)

	out := strings.Builder{}
	out.WriteString("# ")
	out.WriteString(name)
	out.WriteString(" - Script Plugin Scaffold\n\n")

	out.WriteString("## Python\n\n```python\n")
	out.WriteString(pythonScript)
	out.WriteString("\n```\n\n---\n\n")

	out.WriteString("## Node.js\n\n```javascript\n")
	out.WriteString(nodeScript)
	out.WriteString("\n```\n\n---\n\n")

	out.WriteString("## Bash\n\n```bash\n")
	out.WriteString(bashScript)
	out.WriteString("\n```\n")

	return out.String()
}

func toSnakeCase(s string) string {
	result := ""
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result += "_"
			}
			result += string(c + 32)
		} else if c == ' ' || c == '-' {
			result += "_"
		} else {
			result += string(c)
		}
	}
	return result
}

func toPascalCase(s string) string {
	if s == "" {
		return s
	}
	result := ""
	capitalizeNext := true
	for _, c := range s {
		if c == ' ' || c == '-' || c == '_' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext {
			if c >= 'a' && c <= 'z' {
				result += string(c - 32)
			} else {
				result += string(c)
			}
			capitalizeNext = false
		} else {
			result += string(c)
		}
	}
	return result
}

func toKebabCase(s string) string {
	result := ""
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result += "-"
			}
			result += string(c + 32)
		} else if c == ' ' || c == '_' {
			result += "-"
		} else {
			result += string(c)
		}
	}
	return result
}
