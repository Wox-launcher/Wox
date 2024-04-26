package llm

import (
	"context"
	"errors"
	"wox/plugin"
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

type modelProviderName string

var (
	modelProviderNameOpenAI modelProviderName = "openai"
	modelProviderNameGoogle modelProviderName = "google"
	modelProviderNameOllama modelProviderName = "ollama"
	modelProviderNameGroq   modelProviderName = "groq"
)

var modelProviderNames = []modelProviderName{
	modelProviderNameOpenAI,
	modelProviderNameGoogle,
	modelProviderNameOllama,
	modelProviderNameGroq,
}

type model struct {
	DisplayName string
	Name        string
	Provider    modelProviderName
}

type providerConnectContext struct {
	Provider modelProviderName
	api      plugin.API

	ApiKey string
	Host   string // E.g. "https://api.openai.com:8908"
}

type Provider interface {
	Close(ctx context.Context) error
	ChatStream(ctx context.Context, model model, conversations []Conversation) (ProviderChatStream, error)
	Chat(ctx context.Context, model model, conversations []Conversation) (string, error)
	Models(ctx context.Context) ([]model, error)
}

type ProviderChatStream interface {
	Receive(ctx context.Context) (string, error) // will return io.EOF if no more messages
	Close(ctx context.Context)
}

func NewProvider(ctx context.Context, connectContext providerConnectContext) (Provider, error) {
	if connectContext.Provider == modelProviderNameGoogle {
		return NewGoogleProvider(ctx, connectContext), nil
	}
	if connectContext.Provider == modelProviderNameOpenAI {
		return NewOpenAIClient(ctx, connectContext), nil
	}
	if connectContext.Provider == modelProviderNameOllama {
		return NewOllamaProvider(ctx, connectContext), nil
	}
	if connectContext.Provider == modelProviderNameGroq {
		return NewGroqProvider(ctx, connectContext), nil
	}

	return nil, errors.New("unknown model provider")
}
