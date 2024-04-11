package chatgpt

import (
	"context"
	"errors"
)

type chatgptModelProviderName string

var (
	chatgptModelProviderNameOpenAI chatgptModelProviderName = "openai"
	chatgptModelProviderNameGoogle chatgptModelProviderName = "google"
	chatgptModelProviderNameOllama chatgptModelProviderName = "ollama"
)

var chatgptModelProviderNames = []chatgptModelProviderName{
	chatgptModelProviderNameOpenAI,
	chatgptModelProviderNameGoogle,
	chatgptModelProviderNameOllama,
}

type Provider interface {
	Connect(ctx context.Context) error
	Close(ctx context.Context) error
	ChatStream(ctx context.Context, model chatgptModel, conversations []Conversation) (ProviderChatStream, error)
	Chat(ctx context.Context, model chatgptModel, conversations []Conversation) (string, error)
	Models(ctx context.Context) ([]chatgptModel, error)
}

type ProviderChatStream interface {
	Receive() (string, error) // will return io.EOF if no more messages
	Close()
}

func NewProvider(ctx context.Context, apiKey string, provider chatgptModelProviderName) (Provider, error) {
	if provider == chatgptModelProviderNameGoogle {
		return NewGoogleProvider(ctx, apiKey)
	}
	if provider == chatgptModelProviderNameOpenAI {
		return NewOpenAIClient(ctx, apiKey), nil
	}
	if provider == chatgptModelProviderNameOllama {
		return NewOllamaProvider(ctx, apiKey), nil
	}

	return nil, errors.New("unknown model provider")
}
