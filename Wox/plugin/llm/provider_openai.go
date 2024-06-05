package llm

import (
	"context"
	"github.com/sashabaranov/go-openai"
	"io"
)

type OpenAIProvider struct {
	connectContext ProviderConnectContext
	client         *openai.Client
}

type OpenAIProviderStream struct {
	stream        *openai.ChatCompletionStream
	conversations []Conversation
}

func NewOpenAIClient(ctx context.Context, connectContext ProviderConnectContext) Provider {
	return &OpenAIProvider{connectContext: connectContext}
}

func (o *OpenAIProvider) Close(ctx context.Context) error {
	return nil
}

func (o *OpenAIProvider) ensureClient(ctx context.Context) error {
	if o.client == nil {
		o.client = openai.NewClient(o.connectContext.ApiKey)
	}

	return nil
}

func (o *OpenAIProvider) ChatStream(ctx context.Context, model Model, conversations []Conversation) (ChatStream, error) {
	if ensureClientErr := o.ensureClient(ctx); ensureClientErr != nil {
		return nil, ensureClientErr
	}

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

func (o *OpenAIProvider) Models(ctx context.Context) ([]Model, error) {
	return []Model{
		{
			DisplayName: "chatgpt-3.5-turbo",
			Name:        "gpt-3.5-turbo",
			Provider:    ModelProviderNameOpenAI,
		},
	}, nil
}

func (s *OpenAIProviderStream) Receive(ctx context.Context) (string, error) {
	response, err := s.stream.Recv()
	if err != nil {
		s.stream.Close()

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
