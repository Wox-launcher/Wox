package chatgpt

import (
	"context"
	"github.com/sashabaranov/go-openai"
	"io"
)

type OpenAIClient struct {
	client *openai.Client
}

type OpenAIClientStream struct {
	stream        *openai.ChatCompletionStream
	conversations []Conversation
}

func NewOpenAIClient(apiKey string) Client {
	return &OpenAIClient{client: openai.NewClient(apiKey)}
}

func (c *OpenAIClient) ChatStream(ctx context.Context, model chatgptModel, conversations []Conversation) (ClientChatStream, error) {
	createdStream, createErr := c.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Stream:   true,
		Model:    model.Name,
		Messages: c.convertConversations(conversations),
	})
	if createErr != nil {
		return nil, createErr
	}

	return &OpenAIClientStream{conversations: conversations, stream: createdStream}, nil
}

func (c *OpenAIClient) Chat(ctx context.Context, model chatgptModel, conversations []Conversation) (string, error) {
	resp, createErr := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    model.Name,
		Messages: c.convertConversations(conversations),
	})
	if createErr != nil {
		return "", createErr
	}

	return resp.Choices[0].Message.Content, nil
}

func (s *OpenAIClientStream) Receive() (string, error) {
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

func (s *OpenAIClientStream) Close() {
	s.stream.Close()
}

func (c *OpenAIClient) convertConversations(conversations []Conversation) []openai.ChatCompletionMessage {
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
