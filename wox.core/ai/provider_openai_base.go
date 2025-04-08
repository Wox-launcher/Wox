package ai

import (
	"context"
	"fmt"
	"io"
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

	util.GetLogger().Debug(ctx, fmt.Sprintf("chat stream with model: %s", model.Name))

	createdStream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model:    model.Name,
		Messages: o.convertConversations(conversations),
		Tools:    o.convertTools(options.Tools),
		ToolChoice: openai.ChatCompletionToolChoiceOptionUnionParam{
			OfAuto: param.Opt[string]{},
		},
	})
	util.GetLogger().Debug(ctx, fmt.Sprintf("chat stream created: %v", createdStream))

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
		convertedTools[i] = openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        tool.Name,
				Description: openai.String(tool.Description),
				Parameters: openai.FunctionParameters{
					"type":       tool.Parameters.Type,
					"properties": tool.Parameters.Properties,
					"required":   tool.Parameters.Required,
				},
			},
		}
	}
	return convertedTools
}

// Receive receives the next message from the stream
func (s *OpenAIBaseProviderStream) Receive(ctx context.Context) (string, error) {
	if !s.stream.Next() {
		if s.stream.Err() != nil {
			return "", s.stream.Err()
		}

		// no more messages
		return "", io.EOF
	}

	chunk := s.stream.Current()
	s.acc.AddChunk(chunk)

	if content, ok := s.acc.JustFinishedContent(); ok {
		return content, io.EOF
	}

	// if using tool calls
	if tool, ok := s.acc.JustFinishedToolCall(); ok {
		println("Tool call stream finished:", tool.Index, tool.Name, tool.Arguments)
	}

	if refusal, ok := s.acc.JustFinishedRefusal(); ok {
		return refusal, io.EOF
	}

	// it's best to use chunks after handling JustFinished events
	if len(chunk.Choices) > 0 {
		return chunk.Choices[0].Delta.Content, nil
	}

	return "", io.EOF
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
			chatMessages = append(chatMessages, openai.ToolMessage(conversation.Text, conversation.Id))
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
