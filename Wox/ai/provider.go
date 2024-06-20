package ai

import (
	"context"
	"errors"
	"wox/setting"
)

type ProviderName string

var (
	ProviderNameOpenAI ProviderName = "openai"
	ProviderNameGoogle ProviderName = "google"
	ProviderNameOllama ProviderName = "ollama"
	ProviderNameGroq   ProviderName = "groq"
)

type Provider interface {
	Close(ctx context.Context) error
	ChatStream(ctx context.Context, model Model, conversations []Conversation) (ChatStream, error)
	Models(ctx context.Context) ([]Model, error)
}

type ChatStreamDataType string

const (
	ChatStreamTypeStreaming ChatStreamDataType = "streaming"
	ChatStreamTypeFinished  ChatStreamDataType = "finished"
	ChatStreamTypeError     ChatStreamDataType = "error"
)

type ChatStreamFunc func(t ChatStreamDataType, data string)

type ChatStream interface {
	Receive(ctx context.Context) (string, error) // will return io.EOF if no more messages
}

func NewProvider(ctx context.Context, providerSetting setting.AIProvider) (Provider, error) {
	if providerSetting.Name == string(ProviderNameGoogle) {
		return NewGoogleProvider(ctx, providerSetting), nil
	}
	if providerSetting.Name == string(ProviderNameOpenAI) {
		return NewOpenAIClient(ctx, providerSetting), nil
	}
	if providerSetting.Name == string(ProviderNameOllama) {
		return NewOllamaProvider(ctx, providerSetting), nil
	}
	if providerSetting.Name == string(ProviderNameGroq) {
		return NewGroqProvider(ctx, providerSetting), nil
	}

	return nil, errors.New("unknown model provider")
}
