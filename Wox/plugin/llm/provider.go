package llm

import (
	"context"
	"errors"
)

type ConversationRole string

var (
	ConversationRoleUser   ConversationRole = "user"
	ConversationRoleSystem ConversationRole = "system"
)

type Conversation struct {
	Role      ConversationRole
	Text      string
	Timestamp int64
}

type ModelProviderName string

var (
	ModelProviderNameOpenAI ModelProviderName = "openai"
	ModelProviderNameGoogle ModelProviderName = "google"
	ModelProviderNameOllama ModelProviderName = "ollama"
	ModelProviderNameGroq   ModelProviderName = "groq"
)

type Model struct {
	DisplayName string
	Name        string
	Provider    ModelProviderName
}

type Provider interface {
	Close(ctx context.Context) error
	ChatStream(ctx context.Context, model Model, conversations []Conversation) (ChatStream, error)
	Chat(ctx context.Context, model Model, conversations []Conversation) (string, error)
	Models(ctx context.Context) ([]Model, error)
}

type ChatStream interface {
	Receive(ctx context.Context) (string, error) // will return io.EOF if no more messages
}

type ProviderConnectContext struct {
	Provider ModelProviderName

	ApiKey string
	Host   string // E.g. "https://api.openai.com:8908"
}

func NewProvider(ctx context.Context, connectContext ProviderConnectContext) (Provider, error) {
	if connectContext.Provider == ModelProviderNameGoogle {
		return NewGoogleProvider(ctx, connectContext), nil
	}
	if connectContext.Provider == ModelProviderNameOpenAI {
		return NewOpenAIClient(ctx, connectContext), nil
	}
	if connectContext.Provider == ModelProviderNameOllama {
		return NewOllamaProvider(ctx, connectContext), nil
	}
	if connectContext.Provider == ModelProviderNameGroq {
		return NewGroqProvider(ctx, connectContext), nil
	}

	return nil, errors.New("unknown model provider")
}
