package system

import (
	"context"
	"fmt"

	"wox/ai"
	"wox/common"
	"wox/plugin"
	"wox/util"

	"github.com/google/uuid"
	"github.com/tmc/langchaingo/jsonschema"
)

// registerPluginBuiltinTools registers builtin tools that depend on the plugin
// manager (and therefore can't live in the wox/ai package without an import
// cycle). It must be called after the plugin manager is initialized.
func registerPluginBuiltinTools(ctx context.Context) {
	ai.GetToolRegistry().Register(common.Tool{
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
	})

	// Wire the ask_user UI hook so the ai package can push questions to the UI
	// without importing wox/plugin.
	ai.SendAIQuestionHook = func(ctx context.Context, questionId string, question string, options []common.AIQuestionOption) {
		plugin.GetPluginManager().GetUI().SendAIQuestion(ctx, questionId, question, options)
	}
}

func testQueryCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	queryStr, _ := args["query"].(string)
	if queryStr == "" {
		return common.ToolResult{}, fmt.Errorf("query is required")
	}
	query := plugin.Query{
		Id:        uuid.NewString(),
		SessionId: util.GetContextSessionId(ctx),
		Type:      plugin.QueryTypeInput,
		RawQuery:  queryStr,
		Search:    queryStr,
	}
	ok := plugin.GetPluginManager().QuerySilent(ctx, query)
	if ok {
		return common.ToolResult{Text: "query executed successfully (single result, default action ran)"}, nil
	}
	return common.ToolResult{Text: "query did not produce a single executable result"}, nil
}
