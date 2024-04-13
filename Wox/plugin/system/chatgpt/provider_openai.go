package chatgpt

import (
	"context"
	"github.com/sashabaranov/go-openai"
	"io"
	"wox/plugin"
)

type OpenAIProvider struct {
	connectContext chatgptProviderConnectContext
	client         *openai.Client
	api            plugin.API
}

type OpenAIProviderStream struct {
	stream        *openai.ChatCompletionStream
	conversations []Conversation
}

func NewOpenAIClient(ctx context.Context, connectContext chatgptProviderConnectContext, api plugin.API) Provider {
	return &OpenAIProvider{connectContext: connectContext, api: api}
}

func (o *OpenAIProvider) Connect(ctx context.Context) error {
	o.client = openai.NewClient(o.connectContext.ApiKey)
	return nil
}

func (o *OpenAIProvider) Close(ctx context.Context) error {
	return nil
}

func (o *OpenAIProvider) ChatStream(ctx context.Context, model chatgptModel, conversations []Conversation) (ProviderChatStream, error) {
	createdStream, createErr := o.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Stream:   true,
		Model:    model.Name,
		Messages: o.convertConversations(conversations),
	})
	if createErr != nil {
		return nil, createErr
	}

	return &OpenAIProviderStream{conversations: conversations, stream: createdStream}, nil
}

func (o *OpenAIProvider) Chat(ctx context.Context, model chatgptModel, conversations []Conversation) (string, error) {
	resp, createErr := o.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    model.Name,
		Messages: o.convertConversations(conversations),
	})
	if createErr != nil {
		return "", createErr
	}

	return resp.Choices[0].Message.Content, nil
}

func (o *OpenAIProvider) Models(ctx context.Context) ([]chatgptModel, error) {
	return []chatgptModel{
		{
			DisplayName: "chatgpt-3.5-turbo",
			Name:        "gpt-3.5-turbo",
			Provider:    chatgptModelProviderNameOpenAI,
		},
	}, nil
}

func (s *OpenAIProviderStream) Receive(ctx context.Context) (string, error) {
	response, err := s.stream.Recv()
	if err != nil {
		// no more messages
		if err == io.EOF {
			return "", io.EOF
		}

		return "", err
	}
	if len(response.Choices) == 0 {
		return "", io.EOF
	}

	return response.Choices[0].Delta.Content, nil
}

func (s *OpenAIProviderStream) Close(ctx context.Context) {
	s.stream.Close()
}

func (o *OpenAIProvider) convertConversations(conversations []Conversation) []openai.ChatCompletionMessage {
	var chatMessages []openai.ChatCompletionMessage
	for _, conversation := range conversations {
		role := ""
		if conversation.Role == ConversationRoleUser {
			role = openai.ChatMessageRoleUser
		}
		if conversation.Role == ConversationRoleSystem {
			role = openai.ChatMessageRoleSystem
		}
		if role == "" {
			return nil
		}

		chatMessages = append(chatMessages, openai.ChatCompletionMessage{
			Role:    role,
			Content: conversation.Text,
		})
	}

	return chatMessages
}
