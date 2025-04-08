package ai

import (
	"context"
	"wox/common"
	"wox/setting"
)

func init() {
	providerFactories["openrouter"] = NewOpenRouterProvider
}

type OpenRouterProvider struct {
	*OpenAIBaseProvider
}

func (p *OpenRouterProvider) GetIcon() common.WoxImage {
	return common.WoxImage{}
}

func NewOpenRouterProvider(ctx context.Context, connectContext setting.AIProvider) Provider {
	if connectContext.Host == "" {
		connectContext.Host = "https://openrouter.ai/api/v1"
	}

	return &OpenRouterProvider{
		OpenAIBaseProvider: NewOpenAIBaseProvider(connectContext),
	}
}
