package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"wox/common"
	"wox/setting"
	"wox/util"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/pagination"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/packages/ssestream"
)

// OpenAIBaseProvider is the base provider for all OpenAI compatible providers
type OpenAIBaseProvider struct {
	connectContext setting.AIProvider
}

// OpenAIBaseProviderStream represents a stream from OpenAI compatible providers
type OpenAIBaseProviderStream struct {
	stream        *ssestream.Stream[openai.ChatCompletionChunk]
	conversations []common.Conversation
	acc           openai.ChatCompletionAccumulator
}

// NewOpenAIBaseProvider creates a new OpenAI base provider
func NewOpenAIBaseProvider(connectContext setting.AIProvider) *OpenAIBaseProvider {
	return &OpenAIBaseProvider{connectContext: connectContext}
}

// ChatStream starts a chat stream with the OpenAI compatible provider
func (o *OpenAIBaseProvider) ChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions) (ChatStream, error) {
	client := o.getClient(ctx)

	util.GetLogger().Debug(ctx, fmt.Sprintf("AI: chat stream with model: %s, conversations: %d, tools: %d", model.Name, len(conversations), len(options.Tools)))
	// 记录对话内容的摘要
	for i, conv := range conversations {
		util.GetLogger().Debug(ctx, fmt.Sprintf("AI: conversation[%d] - role: %s, text: %s, toolCallID: %s",
			i, conv.Role, truncateString(conv.Text, 100), conv.ToolCallID))
	}

	// 记录工具信息
	for i, tool := range options.Tools {
		util.GetLogger().Debug(ctx, fmt.Sprintf("AI: tool[%d] - name: %s, description: %s",
			i, tool.Name, truncateString(tool.Description, 100)))
	}

	var createdStream *ssestream.Stream[openai.ChatCompletionChunk]
	if len(options.Tools) > 0 {
		chatParams := openai.ChatCompletionNewParams{
			Model:    model.Name,
			Messages: o.convertConversations(conversations),
			Tools:    o.convertTools(options.Tools),
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

func (o *OpenAIBaseProvider) convertTools(tools []common.MCPTool) []openai.ChatCompletionToolParam {
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
	convertedTools := make([]openai.ChatCompletionToolParam, len(tools))
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

		convertedTools[i] = openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters:  openai.FunctionParameters(parametersMap),
			},
		}
	}
	return convertedTools
}

// Receive receives the next message from the stream
func (s *OpenAIBaseProviderStream) Receive(ctx context.Context) (string, common.ChatStreamDataType, error) {
	if !s.stream.Next() {
		if s.stream.Err() != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("AI: Stream error: %v", s.stream.Err()))
			return "", common.ChatStreamTypeError, s.stream.Err()
		}

		// no more messages
		util.GetLogger().Debug(ctx, "AI: Stream ended")
		return "", common.ChatStreamTypeFinished, nil
	}

	chunk := s.stream.Current()
	// skip empty chunk, maybe invoke receive too fast
	if s.isChunkEmpty(chunk) {
		return "", common.ChatStreamTypeStreaming, nil
	}

	s.acc.AddChunk(chunk)
	util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Received chunk - id: %s, model: %s, choices count: %d, acc content: %s", chunk.ID, chunk.Model, len(chunk.Choices), s.acc.ChatCompletion.RawJSON()))

	if content, ok := s.acc.JustFinishedContent(); ok {
		util.GetLogger().Debug(ctx, fmt.Sprintf("AI: acc just finished content: %s", content))
		return content, common.ChatStreamTypeFinished, nil
	}

	if tool, ok := s.acc.JustFinishedToolCall(); ok {
		util.GetLogger().Debug(ctx, fmt.Sprintf("AI: acc just finished tool call: index: %d, id: %s, name: %s, arguments: %s", tool.Index, tool.Id, tool.Name, tool.Arguments))

		toolcallInfo := common.ToolCallInfo{
			Id:             tool.Id,
			Name:           tool.Name,
			Arguments:      map[string]any{},
			Status:         common.ToolCallStatusPending,
			Response:       "",
			StartTimestamp: util.GetSystemTimestamp(),
		}

		// try to unmarshal arguments to map if possible
		var argsMap map[string]any
		unmarshalErr := json.Unmarshal([]byte(tool.Arguments), &argsMap)
		if unmarshalErr == nil {
			toolcallInfo.Arguments = argsMap
		} else {
			util.GetLogger().Error(ctx, fmt.Sprintf("AI: Failed to unmarshal tool call arguments: %s", unmarshalErr.Error()))
			toolcallInfo.Arguments = map[string]any{}
		}

		// marshal tool call info to json
		toolCallJSON, marshalErr := json.Marshal(toolcallInfo)
		if marshalErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("AI: Failed to marshal tool call info: %s", marshalErr.Error()))
			return "", common.ChatStreamTypeError, marshalErr
		}

		util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Returning tool call: %s", toolCallJSON))
		return string(toolCallJSON), common.ChatStreamTypeToolCall, nil
	}

	if refusal, ok := s.acc.JustFinishedRefusal(); ok {
		util.GetLogger().Debug(ctx, fmt.Sprintf("AI: acc just finished refusal: %s", refusal))
		return refusal, common.ChatStreamTypeFinished, nil
	}

	// parse tool from delta
	var toolCallInfo common.ToolCallInfo
	if len(chunk.Choices) > 0 && len(chunk.Choices[0].Delta.ToolCalls) > 0 {
		util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Found tool call in delta - count: %d", len(chunk.Choices[0].Delta.ToolCalls)))
		firstToolCall := chunk.Choices[0].Delta.ToolCalls[0]
		toolCallInfo = common.ToolCallInfo{
			Id:             firstToolCall.ID,
			Name:           firstToolCall.Function.Name,
			Arguments:      map[string]any{},
			Status:         common.ToolCallStatusPending,
			Response:       "",
			StartTimestamp: util.GetSystemTimestamp(),
		}

		// try to unmarshal arguments to map if possible
		var argsMap map[string]any
		unmarshalErr := json.Unmarshal([]byte(firstToolCall.Function.Arguments), &argsMap)
		if unmarshalErr == nil {
			toolCallInfo.Arguments = argsMap
		} else {
			util.GetLogger().Error(ctx, fmt.Sprintf("AI: Failed to unmarshal tool call arguments: %s", unmarshalErr.Error()))
		}
	}
	if toolCallInfo.Name != "" {
		toolCallData, marshalErr := json.Marshal(toolCallInfo)
		if marshalErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("AI: Failed to marshal tool call info: %s", marshalErr.Error()))
			return "", common.ChatStreamTypeError, marshalErr
		}

		util.GetLogger().Debug(ctx, fmt.Sprintf("AI: Returning tool call from delta: %s", toolCallData))
		return string(toolCallData), common.ChatStreamTypeToolCall, nil
	}

	// it's best to use chunks after handling JustFinished events
	if len(chunk.Choices) > 0 {
		content := chunk.Choices[0].Delta.Content
		return content, common.ChatStreamTypeStreaming, nil
	}

	return "", common.ChatStreamTypeFinished, nil
}

func (s *OpenAIBaseProviderStream) isChunkEmpty(chunk openai.ChatCompletionChunk) bool {
	if len(chunk.Choices) == 0 {
		return true
	}
	if chunk.Choices[0].Delta.Content == "" && chunk.Choices[0].Delta.Refusal == "" && len(chunk.Choices[0].Delta.ToolCalls) == 0 {
		return true
	}

	return false
}

// convertConversations converts the conversations to OpenAI format
func (o *OpenAIBaseProvider) convertConversations(conversations []common.Conversation) []openai.ChatCompletionMessageParamUnion {
	util.GetLogger().Debug(context.Background(), fmt.Sprintf("AI: Converting %d conversations to OpenAI format", len(conversations)))

	var chatMessages []openai.ChatCompletionMessageParamUnion
	for i, conversation := range conversations {
		util.GetLogger().Debug(context.Background(), fmt.Sprintf("AI: Converting conversation %d: Role=%s, Text=%s, ToolCallID=%s", i, conversation.Role, truncateString(conversation.Text, 50), conversation.ToolCallID))

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
			util.GetLogger().Debug(context.Background(), fmt.Sprintf("AI: Adding tool message: %s, ID: %s",
				truncateString(conversation.Text, 50), conversation.ToolCallID))
			chatMessages = append(chatMessages, openai.ToolMessage(conversation.Text, conversation.ToolCallID))
		}
	}

	return chatMessages
}

// getClient returns an OpenAI client
func (o *OpenAIBaseProvider) getClient(ctx context.Context) openai.Client {
	return openai.NewClient(
		option.WithBaseURL(o.connectContext.Host),
		option.WithAPIKey(o.connectContext.ApiKey),
		option.WithHTTPClient(util.GetHTTPClient(ctx)),
	)
}

// truncateString truncates a string to the given length and adds ellipsis if needed
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
