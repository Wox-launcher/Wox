package ai

import (
	"context"
	"wox/common"
	"wox/setting"
)

func init() {
	providerFactories["openai"] = NewOpenAIClient
}

type OpenAIProvider struct {
	*OpenAIBaseProvider
}

func (p *OpenAIProvider) GetIcon() common.WoxImage {
	return common.WoxImage{}
}

func NewOpenAIClient(ctx context.Context, connectContext setting.AIProvider) Provider {
	if connectContext.Host == "" {
		connectContext.Host = "https://api.openai.com/v1"
	}

	return &OpenAIProvider{
		OpenAIBaseProvider: NewOpenAIBaseProvider(connectContext),
	}
}
