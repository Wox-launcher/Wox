package ai

import (
	"context"
	"errors"
	"wox/entity"
	"wox/setting"
)

type Provider interface {
	ChatStream(ctx context.Context, model entity.Model, conversations []entity.Conversation) (ChatStream, error)
	Models(ctx context.Context) ([]entity.Model, error)
	Ping(ctx context.Context) error
}

type ChatStream interface {
	Receive(ctx context.Context) (string, error) // will return io.EOF if no more messages
}

func NewProvider(ctx context.Context, providerSetting setting.AIProvider) (Provider, error) {
	if providerSetting.Name == string(entity.ProviderNameGoogle) {
		return NewGoogleProvider(ctx, providerSetting), nil
	}
	if providerSetting.Name == string(entity.ProviderNameOpenAI) {
		return NewOpenAIClient(ctx, providerSetting), nil
	}
	if providerSetting.Name == string(entity.ProviderNameOllama) {
		return NewOllamaProvider(ctx, providerSetting), nil
	}
	if providerSetting.Name == string(entity.ProviderNameGroq) {
		return NewGroqProvider(ctx, providerSetting), nil
	}

	return nil, errors.New("unknown model provider")
}
