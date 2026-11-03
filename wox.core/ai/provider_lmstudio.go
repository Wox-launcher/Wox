package ai

import (
	"context"
	"wox/common"
	"wox/common/icons"
	"wox/setting"
)

func init() {
	providerFactories[common.ProviderNameLMStudio] = NewLMStudioProvider
}

// LMStudioProvider uses LM Studio's OpenAI-compatible API.
type LMStudioProvider struct {
	*OpenAIBaseProvider
}

const lmStudioDefaultHost = "http://localhost:1234/v1"

func (p *LMStudioProvider) GetIcon() common.WoxImage {
	return icons.Get(icons.BrandLMStudio)
}

func (p *LMStudioProvider) GetDefaultHost() string {
	return lmStudioDefaultHost
}

// NewLMStudioProvider connects to the local server unless a host is specified.
func NewLMStudioProvider(_ context.Context, config setting.AIProvider) Provider {
	if config.Host == "" {
		config.Host = lmStudioDefaultHost
	}
	return &LMStudioProvider{OpenAIBaseProvider: NewOpenAIBaseProvider(config)}
}
