package ai

import (
	"context"
	"errors"
	"wox/common"
	"wox/setting"
)

var providerFactories = map[common.ProviderName]func(ctx context.Context, providerSetting setting.AIProvider) Provider{}

type Provider interface {
	GetIcon() common.WoxImage
	ChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions) (ChatStream, error)
	Models(ctx context.Context) ([]common.Model, error)
	Ping(ctx context.Context) error
}

type ChatStream interface {
	Receive(ctx context.Context) (string, common.ChatStreamDataType, error)
}

func NewProvider(ctx context.Context, providerSetting setting.AIProvider) (Provider, error) {
	if factory, ok := providerFactories[providerSetting.Name]; ok {
		return factory(ctx, providerSetting), nil
	}

	return nil, errors.New("unknown model provider")
}

func GetAllProviders() []common.AIProviderInfo {
	providers := []common.AIProviderInfo{}
	for name, factory := range providerFactories {
		provider := factory(context.Background(), setting.AIProvider{Name: name})
		providers = append(providers, common.AIProviderInfo{Name: name, Icon: provider.GetIcon()})
	}
	return providers
}
