package ai

import (
	"context"
	"errors"
	"wox/common"
	"wox/setting"
)

type Provider interface {
	ChatStream(ctx context.Context, model common.Model, conversations []common.Conversation) (ChatStream, error)
	Models(ctx context.Context) ([]common.Model, error)
	Ping(ctx context.Context) error
}

type ChatStream interface {
	Receive(ctx context.Context) (string, error) // will return io.EOF if no more messages
}

func NewProvider(ctx context.Context, providerSetting setting.AIProvider) (Provider, error) {
	if providerSetting.Name == string(common.ProviderNameGoogle) {
		return NewGoogleProvider(ctx, providerSetting), nil
	}
	if providerSetting.Name == string(common.ProviderNameOpenAI) {
		return NewOpenAIClient(ctx, providerSetting), nil
	}
	if providerSetting.Name == string(common.ProviderNameOllama) {
		return NewOllamaProvider(ctx, providerSetting), nil
	}
	if providerSetting.Name == string(common.ProviderNameGroq) {
		return NewGroqProvider(ctx, providerSetting), nil
	}

	return nil, errors.New("unknown model provider")
}
