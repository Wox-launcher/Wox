package ai

import (
	"context"
	"fmt"
	"strings"
	"wox/common"
	"wox/setting"

	"github.com/openai/openai-go/v3/option"
)

func init() {
	providerFactories["siliconflow"] = NewSiliconFlowProvider
}

type SiliconFlowProvider struct {
	*OpenAIBaseProvider
}

const siliconFlowDefaultHost = "https://api.siliconflow.cn/v1"

func (p *SiliconFlowProvider) GetIcon() common.WoxImage {
	return common.NewWoxImageSvg(`<svg height="1em" style="flex:none;line-height:1" viewBox="0 0 24 24" width="1em" xmlns="http://www.w3.org/2000/svg"><title>SiliconCloud</title><path clip-rule="evenodd" d="M22.956 6.521H12.522c-.577 0-1.044.468-1.044 1.044v3.13c0 .577-.466 1.044-1.043 1.044H1.044c-.577 0-1.044.467-1.044 1.044v4.174C0 17.533.467 18 1.044 18h10.434c.577 0 1.044-.467 1.044-1.043v-3.13c0-.578.466-1.044 1.043-1.044h9.391c.577 0 1.044-.467 1.044-1.044V7.565c0-.576-.467-1.044-1.044-1.044z" fill="#6E29F6" fill-rule="evenodd"></path></svg>`)
}

func (p *SiliconFlowProvider) GetDefaultHost() string {
	return siliconFlowDefaultHost
}

func (p *SiliconFlowProvider) ChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions) (ChatStream, error) {
	if reason := siliconFlowNonChatModelReason(model.Name); reason != "" {
		return nil, fmt.Errorf("%s", reason)
	}
	return p.OpenAIBaseProvider.ChatStream(ctx, model, conversations, options)
}

func siliconFlowNonChatModelReason(name string) string {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "bge-") || strings.Contains(lower, "embedding") || strings.Contains(lower, "rerank") {
		return fmt.Sprintf("%s is a SiliconFlow embedding or rerank model and cannot be used for chat. Choose a chat model such as Qwen or DeepSeek.", name)
	}
	return ""
}

func NewSiliconFlowProvider(ctx context.Context, connectContext setting.AIProvider) Provider {
	if connectContext.Host == "" {
		connectContext.Host = siliconFlowDefaultHost
	}

	return &SiliconFlowProvider{
		OpenAIBaseProvider: NewOpenAIBaseProviderWithOptions(connectContext, OpenAIBaseProviderOptions{
			// SiliconFlow's /v1/models catalog includes embeddings, rerankers, and
			// image models. Those IDs 400 on chat/completions, and BAAI embeddings
			// sort first so they become the accidental default.
			ModelsListOptions: []option.RequestOption{option.WithQuery("sub_type", "chat")},
			ChatRequestOptions: func(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions) []option.RequestOption {
				// SiliconFlow uses enable_thinking instead of OpenAI reasoning fields.
				// Only send it when the user picks an explicit mode so models that
				// reject the parameter keep their provider default.
				switch options.ThinkingMode {
				case common.ChatThinkingModeThinking:
					return []option.RequestOption{option.WithJSONSet("enable_thinking", true)}
				case common.ChatThinkingModeNonThinking:
					return []option.RequestOption{option.WithJSONSet("enable_thinking", false)}
				default:
					return nil
				}
			},
		}),
	}
}
