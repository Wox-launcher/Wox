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

	for i, conv := range conversations {
		util.GetLogger().Debug(ctx, fmt.Sprintf("AI: conversation[%d] - role: %s, text: %s, toolCallID: %s", i, conv.Role, conv.Text, conv.ToolCallInfo.Id))
	}
	for i, tool := range options.Tools {
		util.GetLogger().Debug(ctx, fmt.Sprintf("AI: tool[%d] - name: %s, description: %s", i, tool.Name, tool.Description))
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

func (s *OpenAIBaseProviderStream) Receive(ctx context.Context) (common.ChatStreamData, error) {
	if !s.stream.Next() {
		if s.stream.Err() != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("AI: Stream error: %v", s.stream.Err()))
			return common.ChatStreamData{}, s.stream.Err()
		}

		// check if tool call finished
		var toolCall openai.ChatCompletionMessageToolCall
		// somehow acc.JustFinishedToolCall is not working in my test, so we need to check the last tool call
		// and if that failed, we check the just finished tool call
		if len(s.acc.Choices) > 0 && len(s.acc.Choices[0].Message.ToolCalls) > 0 {
			toolCall = s.acc.Choices[0].Message.ToolCalls[len(s.acc.Choices[0].Message.ToolCalls)-1]
		}
		if toolCall.Function.Name == "" {
			if justFinishedToolCall, ok := s.acc.JustFinishedToolCall(); ok {
				toolCall = openai.ChatCompletionMessageToolCall{
					ID: justFinishedToolCall.Id,
					Function: openai.ChatCompletionMessageToolCallFunction{
						Name:      justFinishedToolCall.Name,
						Arguments: justFinishedToolCall.Arguments,
					},
				}
			}
		}
		if toolCall.Function.Name != "" {
			util.GetLogger().Debug(ctx, "AI: Tool call streaming finished")
			toolCallInfo := common.ToolCallInfo{
				Id:             toolCall.ID,
				Name:           toolCall.Function.Name,
				Delta:          toolCall.Function.Arguments,
				StartTimestamp: util.GetSystemTimestamp(),
			}

			// try to unmarshal tool call arguments if possible
			var argsMap map[string]any
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &argsMap); err == nil {
				toolCallInfo.Arguments = argsMap
				toolCallInfo.Status = common.ToolCallStatusPending
			} else {
				util.GetLogger().Error(ctx, fmt.Sprintf("AI: Failed to unmarshal tool call arguments, json=%s, err: %s", toolCall.Function.Arguments, err.Error()))
				toolCallInfo.Arguments = map[string]any{}
				toolCallInfo.Status = common.ToolCallStatusFailed
				toolCallInfo.Response = err.Error()
			}

			return common.ChatStreamData{
				Type:     common.ChatStreamTypeToolCall,
				Data:     "",
				ToolCall: toolCallInfo,
			}, nil
		}

		// no more messages
		util.GetLogger().Debug(ctx, "AI: Stream ended")
		return common.ChatStreamData{
			Type: common.ChatStreamTypeFinished,
			Data: "",
		}, nil
	}

	chunk := s.stream.Current()
	s.acc.AddChunk(chunk)

	// skip empty chunk, maybe invoke receive too fast
	if s.isChunkEmpty(chunk) {
		return common.ChatStreamData{}, ChatStreamNoContentErr
	}

	// check if tool call streaming
	if len(chunk.Choices[0].Delta.ToolCalls) > 0 {
		toolCall := chunk.Choices[0].Delta.ToolCalls[0]
		toolCallInfo := common.ToolCallInfo{
			Id:             toolCall.ID,
			Name:           toolCall.Function.Name,
			Arguments:      map[string]any{},
			Delta:          toolCall.Function.Arguments,
			Status:         common.ToolCallStatusStreaming,
			StartTimestamp: util.GetSystemTimestamp(),
		}
		return common.ChatStreamData{
			Type:     common.ChatStreamTypeToolCall,
			Data:     "",
			ToolCall: toolCallInfo,
		}, nil
	}

	return common.ChatStreamData{
		Type: common.ChatStreamTypeStreaming,
		Data: chunk.Choices[0].Delta.Content,
	}, nil
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
			chatMessages = append(chatMessages, openai.ToolMessage(conversation.Text, conversation.ToolCallInfo.Id))
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
