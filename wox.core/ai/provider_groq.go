package ai

import (
	"context"
	"wox/common"
	"wox/setting"
)

func init() {
	providerFactories["groq"] = NewGroqProvider
}

type GroqProvider struct {
	*OpenAIBaseProvider
}

func (p *GroqProvider) GetIcon() common.WoxImage {
	return common.WoxImage{}
}

func NewGroqProvider(ctx context.Context, connectContext setting.AIProvider) Provider {
	if connectContext.Host == "" {
		connectContext.Host = "https://api.groq.com/openai/v1"
	}

	return &GroqProvider{
		OpenAIBaseProvider: NewOpenAIBaseProvider(connectContext),
	}
}
