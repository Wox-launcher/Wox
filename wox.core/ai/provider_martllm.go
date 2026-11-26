package ai

import (
	"context"
	"wox/common"
	"wox/setting"
)

func init() {
	providerFactories["martllm"] = NewMartLLMProvider
}

// MartLLMProvider talks to MartLLM's OpenAI-compatible API, which serves models from many vendors under one key.
type MartLLMProvider struct {
	*OpenAIBaseProvider
}

const martLLMDefaultHost = "https://api.martllm.com/v1"

func (p *MartLLMProvider) GetIcon() common.WoxImage {
	return common.NewWoxImageSvg(`<svg fill="none" stroke="var(--wox-theme-icon-color)" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" height="1em" style="flex:none;line-height:1" viewBox="6.3 6 22 22" width="1em" xmlns="http://www.w3.org/2000/svg"><title>MartLLM</title><path d="M9 16h4m0 0 5-5m-5 5 5 5m5-11a3 3 0 1 1 0 6 3 3 0 0 1 0-6Zm0 8a3 3 0 1 1 0 6 3 3 0 0 1 0-6Z"></path><circle cx="10" cy="16" r="2.2" fill="var(--wox-theme-icon-color)" stroke="none"></circle></svg>`)
}

func (p *MartLLMProvider) GetDefaultHost() string {
	return martLLMDefaultHost
}

func NewMartLLMProvider(ctx context.Context, connectContext setting.AIProvider) Provider {
	if connectContext.Host == "" {
		connectContext.Host = martLLMDefaultHost
	}

	return &MartLLMProvider{
		OpenAIBaseProvider: NewOpenAIBaseProvider(connectContext),
	}
}
