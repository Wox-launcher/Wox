package tool

import (
	"context"
	"fmt"
	"strings"

	"wox/ai"
	"wox/common"

	"github.com/tmc/langchaingo/jsonschema"
)

func init() {
	ai.GetToolRegistry().Register(LoadToolsTool())
}

// LoadToolsTool lets the model request executable tools for the next loop step.
func LoadToolsTool() common.Tool {
	return common.Tool{
		Name:        ai.LoadToolsToolName,
		Description: "Request executable tools by exact name from the available_tools context. Requested tools become callable in the next model step.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"names": {
					Type:        jsonschema.Array,
					Description: "Exact tool names to load from the available_tools context",
					Items:       &jsonschema.Definition{Type: jsonschema.String},
				},
			},
			Required: []string{"names"},
		},
		Source:   common.ToolSourceBuiltin,
		Callback: loadToolsCallback,
	}
}

func loadToolsCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	_ = ctx

	names := ai.ParseLoadToolNames(args)
	if len(names) == 0 {
		return common.ToolResult{}, fmt.Errorf("name or names is required")
	}

	loaded := []string{}
	missing := []string{}
	for _, name := range names {
		tool, ok := ai.GetToolRegistry().Get(name)
		if !ok || ai.IsRuntimeOnlyTool(name) {
			missing = append(missing, name)
			continue
		}
		loaded = append(loaded, tool.Name)
	}
	if len(loaded) == 0 {
		return common.ToolResult{}, fmt.Errorf("no requested tools were found: %s", strings.Join(missing, ", "))
	}

	var builder strings.Builder
	builder.WriteString("Loaded tools for the next step: ")
	builder.WriteString(strings.Join(loaded, ", "))
	if len(missing) > 0 {
		builder.WriteString("\nUnavailable tools: ")
		builder.WriteString(strings.Join(missing, ", "))
	}
	return common.ToolResult{Text: builder.String()}, nil
}
