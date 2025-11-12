package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"wox/common"
	"wox/setting"
	"wox/util"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/pagination"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/packages/ssestream"
)

type OpenAIBaseProviderOptions struct {
	Headers map[string]string
}

// OpenAIBaseProvider is the base provider for all OpenAI compatible providers
type OpenAIBaseProvider struct {
	connectContext setting.AIProvider
	options        OpenAIBaseProviderOptions
}

// OpenAIBaseProviderStream represents a stream from OpenAI compatible providers
type OpenAIBaseProviderStream struct {
	stream            *ssestream.Stream[openai.ChatCompletionChunk]
	conversations     []common.Conversation
	acc               openai.ChatCompletionAccumulator
	accumulatedReason string // accumulated reasoning content from chunks
}

// NewOpenAIBaseProvider creates a new OpenAI base provider
func NewOpenAIBaseProvider(connectContext setting.AIProvider) *OpenAIBaseProvider {
	return &OpenAIBaseProvider{connectContext: connectContext}
}

func NewOpenAIBaseProviderWithOptions(connectContext setting.AIProvider, options OpenAIBaseProviderOptions) *OpenAIBaseProvider {
	return &OpenAIBaseProvider{connectContext: connectContext, options: options}
}

// ChatStream starts a chat stream with the OpenAI compatible provider
func (o *OpenAIBaseProvider) ChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions) (ChatStream, error) {
	client := o.getClient(ctx)

	util.GetLogger().Debug(ctx, fmt.Sprintf("AI: chat stream with model: %s, conversations: %d, tools: %d", model.Name, len(conversations), len(options.Tools)))

	for i, conv := range conversations {
		util.GetLogger().Debug(ctx, fmt.Sprintf("AI: conversation[%d] - role: %s, text: %s, toolCallID: %s", i, conv.Role, conv.Text, conv.ToolCallInfo.Id))
	}
	convertedTools := o.convertTools(options.Tools)
	for i, tool := range convertedTools {
		if function := tool.GetFunction(); function != nil {
			util.GetLogger().Debug(ctx, fmt.Sprintf("AI: converted tool[%d] name: %s, paramters: %v", i, function.Name, function.Parameters))
		}
	}

	var createdStream *ssestream.Stream[openai.ChatCompletionChunk]
	if len(options.Tools) > 0 {
		chatParams := openai.ChatCompletionNewParams{
			Model:    model.Name,
			Messages: o.convertConversations(conversations),
			Tools:    convertedTools,
			ToolChoice: openai.ChatCompletionToolChoiceOptionUnionParam{
				OfAuto: param.Opt[string]{},
			},
		}
		createdStream = client.Chat.Completions.NewStreaming(ctx, chatParams)
	} else {
		createdStream = client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
			Model:    model.Name,
			Messages: o.convertConversations(conversations),
		})
	}

	return &OpenAIBaseProviderStream{conversations: conversations, stream: createdStream}, nil
}

// Models returns the list of available models from the OpenAI compatible provider
func (o *OpenAIBaseProvider) Models(ctx context.Context) ([]common.Model, error) {
	client := o.getClient(ctx)
	models, err := client.Models.List(ctx)
	if err != nil {
		return nil, err
	}

	pageAutoPager := pagination.NewPageAutoPager(models, err)
	var openaiModels []common.Model
	for pageAutoPager.Next() {
		model := pageAutoPager.Current()
		openaiModels = append(openaiModels, common.Model{
			Name:     model.ID,
			Provider: common.ProviderName(o.connectContext.Name),
		})
	}

	return openaiModels, nil
}

// Ping checks if the OpenAI compatible provider is available
func (o *OpenAIBaseProvider) Ping(ctx context.Context) error {
	client := o.getClient(ctx)
	_, err := client.Models.List(ctx)
	return err
}

func (o *OpenAIBaseProvider) convertTools(tools []common.MCPTool) []openai.ChatCompletionToolUnionParam {
	/*
		{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        "getCurrentWeather",
				Description: "Get the current weather in a given location",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"rationale": {
							Type:        jsonschema.String,
							Description: "The rationale for choosing this function call with these parameters",
						},
						"location": {
							Type:        jsonschema.String,
							Description: "The city and state, e.g. San Francisco, CA",
						},
						"unit": {
							Type: jsonschema.String,
							Enum: []string{"celsius", "fahrenheit"},
						},
					},
					Required: []string{"rationale", "location"},
				},
			},
		}
	*/
	convertedTools := make([]openai.ChatCompletionToolUnionParam, len(tools))
	for i, tool := range tools {
		parametersMap := make(map[string]any)
		parametersMap["type"] = tool.Parameters.Type

		if tool.Parameters.Properties != nil {
			parametersMap["properties"] = tool.Parameters.Properties
		} else {
			parametersMap["properties"] = map[string]any{}
		}

		if len(tool.Parameters.Required) > 0 {
			parametersMap["required"] = tool.Parameters.Required
		}

		convertedTools[i] = openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        tool.Name,
			Description: openai.String(tool.Description),
			Parameters:  openai.FunctionParameters(parametersMap),
		})
	}
	return convertedTools
}

func (s *OpenAIBaseProviderStream) Receive(ctx context.Context) (common.ChatStreamData, error) {
	if !s.stream.Next() {
		if s.stream.Err() != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("AI: Stream error: %v", s.stream.Err()))
			return common.ChatStreamData{}, s.stream.Err()
		}

		var toolCallInfos []common.ToolCallInfo
		if len(s.acc.Choices) > 0 && len(s.acc.Choices[0].Message.ToolCalls) > 0 {
			toolCalls := s.acc.Choices[0].Message.ToolCalls
			util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Tool call streaming finished, tool calls count: %d", len(toolCalls)))

			for _, toolCall := range toolCalls {
				toolCallInfo := common.ToolCallInfo{
					Id:    toolCall.ID,
					Name:  toolCall.Function.Name,
					Delta: toolCall.Function.Arguments,
				}

				// try to unmarshal tool call arguments if possible
				var argsMap map[string]any
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &argsMap); err == nil {
					toolCallInfo.Arguments = s.normalizeArguments(ctx, toolCall.Function.Name, argsMap)
					toolCallInfo.Status = common.ToolCallStatusPending
				} else {
					util.GetLogger().Error(ctx, fmt.Sprintf("AI: Failed to unmarshal tool call arguments, json=%s, err: %s", toolCall.Function.Arguments, err.Error()))
					toolCallInfo.Arguments = map[string]any{}
					toolCallInfo.Status = common.ToolCallStatusFailed
					toolCallInfo.Response = err.Error()
				}

				toolCallInfos = append(toolCallInfos, toolCallInfo)
			}
		}

		// Combine reasoning and content for final message
		finalContent := s.acc.Choices[0].Message.Content
		if s.accumulatedReason != "" {
			// Format reasoning as markdown blockquote
			reasoningLines := strings.Split(s.accumulatedReason, "\n")
			var formattedReasoning strings.Builder
			for _, line := range reasoningLines {
				formattedReasoning.WriteString("> ")
				formattedReasoning.WriteString(line)
				formattedReasoning.WriteString("\n")
			}

			// Combine reasoning and content
			if finalContent != "" {
				finalContent = formattedReasoning.String() + "\n" + finalContent
			} else {
				finalContent = formattedReasoning.String()
			}
		}

		util.GetLogger().Debug(ctx, "AI: Stream ended, final message received"+finalContent)
		return common.ChatStreamData{
			Status:    common.ChatStreamStatusStreamed,
			Data:      finalContent,
			ToolCalls: toolCallInfos,
		}, nil
	}

	chunk := s.stream.Current()
	util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Received raw chunk: %s", chunk.RawJSON()))

	// Store previous content and reasoning before adding chunk
	var previousContent string
	var previousReasoning string
	if len(s.acc.Choices) > 0 {
		previousContent = s.acc.Choices[0].Message.Content
	}
	previousReasoning = s.accumulatedReason

	// Extract reasoning from current chunk if present
	if len(chunk.Choices) > 0 {
		delta := chunk.Choices[0].Delta

		if reasoningField, exists := delta.JSON.ExtraFields["reasoning"]; exists {
			// The reasoning field is already a JSON string, so we need to unmarshal it
			rawReasoning := reasoningField.Raw()

			// Only process if reasoning is not null
			if rawReasoning != "null" && rawReasoning != "" {
				var reasoningStr string
				if err := json.Unmarshal([]byte(rawReasoning), &reasoningStr); err == nil {
					if reasoningStr != "" {
						s.accumulatedReason += reasoningStr
						util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Extracted reasoning from chunk: %s", reasoningStr))
					}
				} else {
					util.GetLogger().Error(ctx, fmt.Sprintf("AI: Failed to unmarshal reasoning: %s, error: %s", rawReasoning, err.Error()))
				}
			}
		}
	}

	s.acc.AddChunk(chunk)

	// Check if content has changed after adding chunk
	// This handles both regular content and reasoning content (which OpenAI SDK accumulates into Message.Content)
	var currentContent string
	if len(s.acc.Choices) > 0 {
		currentContent = s.acc.Choices[0].Message.Content
	}

	// If neither content nor reasoning has changed and there are no tool calls, skip this chunk
	if currentContent == previousContent && s.accumulatedReason == previousReasoning && s.isChunkEmpty(chunk) {
		return common.ChatStreamData{}, ChatStreamNoContentErr
	}

	// Combine reasoning and content for display
	// Format reasoning as markdown quote (similar to <think> tag handling)
	displayContent := currentContent
	if s.accumulatedReason != "" {
		// Format reasoning as markdown blockquote
		reasoningLines := strings.Split(s.accumulatedReason, "\n")
		var formattedReasoning strings.Builder
		for _, line := range reasoningLines {
			formattedReasoning.WriteString("> ")
			formattedReasoning.WriteString(line)
			formattedReasoning.WriteString("\n")
		}

		// Combine reasoning and content
		if currentContent != "" {
			displayContent = formattedReasoning.String() + "\n" + currentContent
		} else {
			displayContent = formattedReasoning.String()
		}
	}

	streamData := common.ChatStreamData{
		Status: common.ChatStreamStatusStreaming,
		Data:   displayContent,
	}
	var totalToolCallCount = len(s.acc.Choices[0].Message.ToolCalls)
	if totalToolCallCount > 0 {
		var toolCallInfos []common.ToolCallInfo
		for index, toolcall := range s.acc.Choices[0].Message.ToolCalls {
			isLastToolCall := index == totalToolCallCount-1

			// if the toolcall is not the last one, we will set the status to pending, because the tool call streaming is one by one
			// the prev toolcall streaming must be finished before the next one
			status := common.ToolCallStatusStreaming
			if totalToolCallCount > 1 && !isLastToolCall {
				status = common.ToolCallStatusPending
			}

			toolCallInfo := common.ToolCallInfo{
				Id:        toolcall.ID,
				Name:      toolcall.Function.Name,
				Arguments: map[string]any{},
				Delta:     toolcall.Function.Arguments,
				Status:    status,
			}
			toolCallInfos = append(toolCallInfos, toolCallInfo)
		}
		streamData.ToolCalls = toolCallInfos
	}

	return streamData, nil
}

// normalizeArguments normalizes the tool call arguments
// Case 1:
//
//		because we unmarshal the tool call arguments as map[string]any, some types are not correct, E.g. int64 will be unmarshaled as float64
//	 so we need to normalize the types base on the tool call definition
//
// Case 2:
//
//	the model does not always generate valid JSON, and may hallucinate parameters not defined by your function schema.
//
// E.g. {"sequenceNumber": 123} -> {"sequence_number": 123}
//
// Case 3:
//
//	sometimes required arguments are not provided, so we need to add them to the arguments
func (s *OpenAIBaseProviderStream) normalizeArguments(ctx context.Context, toolName string, argsMap map[string]any) map[string]any {
	util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Start normalizing tool call arguments for tool: %s, args: %v", toolName, argsMap))

	var tool common.MCPTool
	mcpTools.Range(func(key string, value []common.MCPTool) bool {
		for _, t := range value {
			if t.Name == toolName {
				tool = t
				return false
			}
		}
		return true
	})

	if tool.Name == "" {
		util.GetLogger().Error(ctx, fmt.Sprintf("AI: Tool not found: %s", toolName))
		return argsMap
	}

	// fix argument types
	for toolRequiredName, param := range tool.Parameters.Properties {
		if param.Type == "integer" {
			// name sometimes is not the same as the tool call argument name, so we need to map the name to the tool call argument name
			// E.g. sequenceNumber -> sequence_number
			for aiReturnName, value := range argsMap {
				if s.isToolCallArgumentNameSame(toolRequiredName, aiReturnName) {
					if f, ok := value.(float64); ok {
						argsMap[toolRequiredName] = int64(f)
						util.GetLogger().Debug(ctx, fmt.Sprintf("AI: argument type fixed %s, from float to int", toolRequiredName))
					}
				}
			}
		}
	}

	// fix required arguments
	for _, requiredName := range tool.Parameters.Required {
		if _, ok := argsMap[requiredName]; !ok {
			// add the required argument to the arguments based on the property definition
			if prop, ok := tool.Parameters.Properties[requiredName]; ok {
				if prop.Type == "string" {
					argsMap[requiredName] = ""
				} else if prop.Type == "integer" {
					argsMap[requiredName] = int64(0)
				} else if prop.Type == "object" {
					argsMap[requiredName] = map[string]any{}
				} else if prop.Type == "array" {
					argsMap[requiredName] = []any{}
				} else if prop.Type == "boolean" {
					argsMap[requiredName] = false
				} else {
					argsMap[requiredName] = nil
				}

				util.GetLogger().Debug(ctx, fmt.Sprintf("AI: required argument %s missing, added with default value: %s", requiredName, argsMap[requiredName]))
			} else {
				argsMap[requiredName] = nil
			}
		}
	}

	util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Normalized tool call arguments successfully, args: %v", argsMap))

	return argsMap
}

func (s *OpenAIBaseProviderStream) isToolCallArgumentNameSame(toolRequiredName string, aiReturnName string) bool {
	if strings.EqualFold(toolRequiredName, aiReturnName) {
		return true
	}

	// name sometimes is not the same as the tool call argument name, so we need to map the name to the tool call argument name
	// E.g. sequenceNumber -> sequence_number
	if strings.EqualFold(strings.ReplaceAll(toolRequiredName, "_", ""), strings.ReplaceAll(aiReturnName, "_", "")) {
		return true
	}

	return false
}

func (s *OpenAIBaseProviderStream) isChunkEmpty(chunk openai.ChatCompletionChunk) bool {
	if len(chunk.Choices) == 0 {
		return true
	}

	delta := chunk.Choices[0].Delta

	// Check regular fields
	if delta.Content != "" || delta.Refusal != "" || len(delta.ToolCalls) > 0 {
		return false
	}

	// Check for reasoning field in ExtraFields (for reasoning models like o1, o3-mini, etc.)
	if reasoningField, exists := delta.JSON.ExtraFields["reasoning"]; exists && reasoningField.Valid() {
		return false
	}

	return true
}

// convertConversations converts the conversations to OpenAI format
func (o *OpenAIBaseProvider) convertConversations(conversations []common.Conversation) []openai.ChatCompletionMessageParamUnion {
	var chatMessages []openai.ChatCompletionMessageParamUnion
	for _, conversation := range conversations {
		if conversation.Role == common.ConversationRoleSystem {
			chatMessages = append(chatMessages, openai.SystemMessage(conversation.Text))
		}
		if conversation.Role == common.ConversationRoleUser {
			chatMessages = append(chatMessages, openai.UserMessage(conversation.Text))
		}
		if conversation.Role == common.ConversationRoleAssistant {
			chatMessages = append(chatMessages, openai.AssistantMessage(conversation.Text))
		}
		if conversation.Role == common.ConversationRoleTool {
			// add tool message first, and then add tool output message
			chatMessages = append(chatMessages, openai.ChatCompletionMessageParamUnion{OfAssistant: &openai.ChatCompletionAssistantMessageParam{
				ToolCalls: []openai.ChatCompletionMessageToolCallUnionParam{
					{
						OfFunction: &openai.ChatCompletionMessageFunctionToolCallParam{
							ID: conversation.ToolCallInfo.Id,
							Function: openai.ChatCompletionMessageFunctionToolCallFunctionParam{
								Name:      conversation.ToolCallInfo.Name,
								Arguments: conversation.ToolCallInfo.Delta,
							},
						},
					},
				},
			}})
			chatMessages = append(chatMessages, openai.ToolMessage(conversation.ToolCallInfo.Response, conversation.ToolCallInfo.Id))
		}
	}

	return chatMessages
}

// getClient returns an OpenAI client
func (o *OpenAIBaseProvider) getClient(ctx context.Context) openai.Client {
	var requestOption = []option.RequestOption{
		option.WithBaseURL(o.connectContext.Host),
		option.WithAPIKey(o.connectContext.ApiKey),
		option.WithHTTPClient(util.GetHTTPClient(ctx)),
	}

	// with custom headers
	if o.options.Headers != nil {
		for k, v := range o.options.Headers {
			requestOption = append(requestOption, option.WithHeaderAdd(k, v))
		}
	}

	return openai.NewClient(requestOption...)
}
