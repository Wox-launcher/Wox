package ai

import (
	"context"
	"wox/common"
	"wox/setting"
)

func init() {
	providerFactories["google"] = NewGoogleProvider
}

type GoogleProvider struct {
	*OpenAIBaseProvider
}

func (p *GoogleProvider) GetIcon() common.WoxImage {
	return common.WoxImage{}
}

func NewGoogleProvider(ctx context.Context, connectContext setting.AIProvider) Provider {
	if connectContext.Host == "" {
		connectContext.Host = "https://generativelanguage.googleapis.com/v1beta/openai/"
	}

	return &GoogleProvider{
		OpenAIBaseProvider: NewOpenAIBaseProvider(connectContext),
	}
}
