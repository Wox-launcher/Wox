package chatgpt

import (
	"context"
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

type chatgptProviderConnectContext struct {
	ApiKey string
	Host   string // E.g. "https://api.openai.com:8908"
}

type Provider interface {
	Connect(ctx context.Context) error
	Close(ctx context.Context) error
	ChatStream(ctx context.Context, model chatgptModel, conversations []Conversation) (ProviderChatStream, error)
	Chat(ctx context.Context, model chatgptModel, conversations []Conversation) (string, error)
	Models(ctx context.Context) ([]chatgptModel, error)
}

type ProviderChatStream interface {
	Receive(ctx context.Context) (string, error) // will return io.EOF if no more messages
	Close(ctx context.Context)
}
