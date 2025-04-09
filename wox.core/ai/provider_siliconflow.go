package ai

import (
	"context"
	"wox/common"
	"wox/setting"
)

func init() {
	providerFactories["siliconflow"] = NewSiliconFlowProvider
}

type SiliconFlowProvider struct {
	*OpenAIBaseProvider
}

func (p *SiliconFlowProvider) GetIcon() common.WoxImage {
	return common.WoxImage{}
}

func NewSiliconFlowProvider(ctx context.Context, connectContext setting.AIProvider) Provider {
	if connectContext.Host == "" {
		connectContext.Host = "https://api.siliconflow.cn/v1"
	}

	return &SiliconFlowProvider{
		OpenAIBaseProvider: NewOpenAIBaseProvider(connectContext),
	}
}
