package ai

import (
	"context"
	"wox/common"
	"wox/setting"
)

func init() {
	providerFactories["ollama"] = NewOllamaProvider
}

type OllamaProvider struct {
	*OpenAIBaseProvider
}

func (p *OllamaProvider) GetIcon() common.WoxImage {
	return common.WoxImage{}
}

func NewOllamaProvider(ctx context.Context, connectContext setting.AIProvider) Provider {
	if connectContext.Host == "" {
		connectContext.Host = "http://localhost:11434/v1"
	}

	return &OllamaProvider{
		OpenAIBaseProvider: NewOpenAIBaseProvider(connectContext),
	}
}
