package tool

import (
	"context"
	"fmt"

	"wox/ai"
	"wox/common"
	"wox/util"

	"github.com/google/uuid"
	"github.com/tmc/langchaingo/jsonschema"
)

func init() {
	ai.GetToolRegistry().Register(AskUserTool())
}

// SendAIQuestionHook is set by the plugin package at startup to deliver questions to the UI.
var SendAIQuestionHook func(ctx context.Context, questionId string, question string, options []common.AIQuestionOption)

// pendingQuestions maps questionId to a response channel resolved by ResolveAIQuestionAnswer.
var pendingQuestions = util.NewHashMap[string, chan string]()

// AskUserTool asks the user for clarification through the chat UI.
func AskUserTool() common.Tool {
	return common.Tool{
		Name:        "ask_user",
		Description: "Ask the user a single question and wait for their response. Rules: 1. Ask only ONE question per call — never bundle multiple questions together. 2. Always provide concrete options for the user to pick from; do not leave options empty. 3. The last option should always be a free-text choice (e.g. title 'Other' or '自定义输入') so the user can type their own answer if none of the predefined options fit. 4. Use this when you need a choice or information only the user can provide; do not use it for things you can determine yourself.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"question": {Type: jsonschema.String, Description: "The question to ask the user"},
				"options": {
					Type:        jsonschema.Array,
					Description: "Concrete choices for the user to pick from. Always provide at least 2 options. The last option should be a free-text fallback (e.g. title 'Other' or '自定义输入') so the user can type their own answer.",
					Items: &jsonschema.Definition{
						Type: jsonschema.Object,
						Properties: map[string]jsonschema.Definition{
							"value":       {Type: jsonschema.String, Description: "Stable value returned to the assistant when selected"},
							"title":       {Type: jsonschema.String, Description: "Primary label shown to the user"},
							"subTitle":    {Type: jsonschema.String, Description: "Optional secondary text shown below the title"},
							"recommended": {Type: jsonschema.Boolean, Description: "Whether this option should be visually marked as recommended"},
							"extra":       {Type: jsonschema.Object, Description: "Optional string metadata for future UI hints"},
						},
						Required: []string{"title"},
					},
				},
			},
			Required: []string{"question"},
		},
		Source:   common.ToolSourceBuiltin,
		Callback: askUserCallback,
	}
}

func askUserCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	question, _ := args["question"].(string)
	if question == "" {
		return common.ToolResult{}, fmt.Errorf("question is required")
	}

	options := parseAIQuestionOptions(args["options"])

	questionId := uuid.NewString()
	responseCh := make(chan string, 1)
	pendingQuestions.Store(questionId, responseCh)
	defer pendingQuestions.Delete(questionId)

	if SendAIQuestionHook != nil {
		SendAIQuestionHook(ctx, questionId, question, options)
	} else {
		return common.ToolResult{}, fmt.Errorf("ask_user is not available: UI hook not configured")
	}

	select {
	case answer := <-responseCh:
		return common.ToolResult{Text: answer}, nil
	case <-ctx.Done():
		return common.ToolResult{}, fmt.Errorf("ask_user cancelled: %w", ctx.Err())
	}
}

// parseAIQuestionOptions accepts structured options while tolerating legacy string options.
func parseAIQuestionOptions(raw any) []common.AIQuestionOption {
	rawOptions, ok := raw.([]any)
	if !ok {
		return nil
	}

	var options []common.AIQuestionOption
	for _, rawOption := range rawOptions {
		switch option := rawOption.(type) {
		case string:
			if option != "" {
				options = append(options, common.AIQuestionOption{Value: option, Title: option})
			}
		case map[string]any:
			parsed := common.AIQuestionOption{
				Value:       getStringArg(option, "value", "Value"),
				Title:       getStringArg(option, "title", "Title"),
				SubTitle:    getStringArg(option, "subTitle", "SubTitle", "subtitle", "Subtitle"),
				Recommended: getBoolArg(option, "recommended", "Recommended"),
				Extra:       getStringMapArg(option, "extra", "Extra"),
			}
			if parsed.Value == "" {
				parsed.Value = parsed.Title
			}
			if parsed.Title == "" {
				parsed.Title = parsed.Value
			}
			if parsed.Title != "" {
				options = append(options, parsed)
			}
		}
	}
	return options
}

func getStringArg(data map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := data[key].(string); ok {
			return value
		}
	}
	return ""
}

func getBoolArg(data map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := data[key].(bool); ok {
			return value
		}
	}
	return false
}

func getStringMapArg(data map[string]any, keys ...string) map[string]string {
	for _, key := range keys {
		rawMap, ok := data[key].(map[string]any)
		if !ok {
			continue
		}

		parsed := map[string]string{}
		for k, v := range rawMap {
			if value, ok := v.(string); ok {
				parsed[k] = value
			}
		}
		if len(parsed) > 0 {
			return parsed
		}
	}
	return nil
}

// ResolveAIQuestionAnswer resolves a pending ask_user request from the UI route.
func ResolveAIQuestionAnswer(questionId string, answer string) {
	if ch, ok := pendingQuestions.Load(questionId); ok {
		select {
		case ch <- answer:
		default:
			// channel already consumed or buffer full; ignore.
		}
	}
}
