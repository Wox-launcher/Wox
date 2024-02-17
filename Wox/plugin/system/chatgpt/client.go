package chatgpt

import (
	"context"
	"errors"
)

type Client interface {
	ChatStream(ctx context.Context, model chatgptModel, conversations []Conversation) (ClientChatStream, error)
	Chat(ctx context.Context, model chatgptModel, conversations []Conversation) (string, error)
}

type ClientChatStream interface {
	Receive() (string, error) // will return io.EOF if no more messages
	Close()
}

func NewClient(ctx context.Context, apiKey string, model chatgptModel) (Client, error) {
	if model.Provider == chatgptModelProviderGoogle {
		return NewGoogleClient(ctx, apiKey)
	}
	if model.Provider == chatgptModelProviderOpenAI {
		return NewOpenAIClient(apiKey), nil
	}

	return nil, errors.New("unknown model provider")
}
