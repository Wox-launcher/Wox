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

const openAIDefaultHost = "https://api.openai.com/v1"

func (p *OpenAIProvider) GetIcon() common.WoxImage {
	return openAIIcon()
}

func (p *OpenAIProvider) GetDefaultHost() string {
	return openAIDefaultHost
}

func NewOpenAIClient(ctx context.Context, connectContext setting.AIProvider) Provider {
	if connectContext.Host == "" {
		connectContext.Host = openAIDefaultHost
	}

	return &OpenAIProvider{
		OpenAIBaseProvider: NewOpenAIBaseProvider(connectContext),
	}
}
