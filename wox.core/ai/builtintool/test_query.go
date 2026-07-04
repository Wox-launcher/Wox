package tool

import (
	"context"
	"fmt"

	"wox/ai"
	"wox/common"

	"github.com/tmc/langchaingo/jsonschema"
)

// TestQueryHook is wired by the system chat plugin because silent queries need plugin manager access.
var TestQueryHook func(ctx context.Context, query string) (string, error)

func init() {
	ai.GetToolRegistry().Register(TestQueryTool())
}

// TestQueryTool executes a silent Wox query to verify plugin behavior.
func TestQueryTool() common.Tool {
	return common.Tool{
		Name:        "test_query",
		Description: "Execute a silent Wox query to verify a plugin works. Returns whether exactly one result was found and executed successfully.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"query": {Type: jsonschema.String, Description: "Raw query string including any trigger keyword, e.g. '> 1+1'"},
			},
			Required: []string{"query"},
		},
		Source:   common.ToolSourceBuiltin,
		Callback: testQueryCallback,
	}
}

func testQueryCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return common.ToolResult{}, fmt.Errorf("query is required")
	}
	if TestQueryHook == nil {
		return common.ToolResult{}, fmt.Errorf("test_query is not available: plugin hook not configured")
	}

	result, err := TestQueryHook(ctx, query)
	if err != nil {
		return common.ToolResult{}, err
	}
	return common.ToolResult{Text: result}, nil
}
