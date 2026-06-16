package ai

import (
	"context"
	"wox/common"
	"wox/setting"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

var miniMaxModels = []string{
	"MiniMax-M2.5",
	"MiniMax-M2.5-highspeed",
}

func init() {
	providerFactories["minimax"] = NewMiniMaxProvider
}

type MiniMaxProvider struct {
	*OpenAIBaseProvider
}

const miniMaxDefaultHost = "https://api.minimaxi.com/v1"

func (p *MiniMaxProvider) GetIcon() common.WoxImage {
	return common.NewWoxImageSvg(`<svg height="1em" style="flex:none;line-height:1" viewBox="0 0 24 24" width="1em" xmlns="http://www.w3.org/2000/svg"><title>Minimax</title><defs><linearGradient id="lobe-icons-minimax-fill" x1="0%" x2="100.182%" y1="50.057%" y2="50.057%"><stop offset="0%" stop-color="#E2167E"></stop><stop offset="100%" stop-color="#FE603C"></stop></linearGradient></defs><path d="M16.278 2c1.156 0 2.093.927 2.093 2.07v12.501a.74.74 0 00.744.709.74.74 0 00.743-.709V9.099a2.06 2.06 0 012.071-2.049A2.06 2.06 0 0124 9.1v6.561a.649.649 0 01-.652.645.649.649 0 01-.653-.645V9.1a.762.762 0 00-.766-.758.762.762 0 00-.766.758v7.472a2.037 2.037 0 01-2.048 2.026 2.037 2.037 0 01-2.048-2.026v-12.5a.785.785 0 00-.788-.753.785.785 0 00-.789.752l-.001 15.904A2.037 2.037 0 0113.441 22a2.037 2.037 0 01-2.048-2.026V18.04c0-.356.292-.645.652-.645.36 0 .652.289.652.645v1.934c0 .263.142.506.372.638.23.131.514.131.744 0a.734.734 0 00.372-.638V4.07c0-1.143.937-2.07 2.093-2.07zm-5.674 0c1.156 0 2.093.927 2.093 2.07v11.523a.648.648 0 01-.652.645.648.648 0 01-.652-.645V4.07a.785.785 0 00-.789-.78.785.785 0 00-.789.78v14.013a2.06 2.06 0 01-2.07 2.048 2.06 2.06 0 01-2.071-2.048V9.1a.762.762 0 00-.766-.758.762.762 0 00-.766.758v3.8a2.06 2.06 0 01-2.071 2.049A2.06 2.06 0 010 12.9v-1.378c0-.357.292-.646.652-.646.36 0 .653.29.653.646V12.9c0 .418.343.757.766.757s.766-.339.766-.757V9.099a2.06 2.06 0 012.07-2.048 2.06 2.06 0 012.071 2.048v8.984c0 .419.343.758.767.758.423 0 .766-.339.766-.758V4.07c0-1.143.937-2.07 2.093-2.07z" fill="url(#lobe-icons-minimax-fill)" fill-rule="nonzero"></path></svg>`)
}

func (p *MiniMaxProvider) GetDefaultHost() string {
	return miniMaxDefaultHost
}

func (p *MiniMaxProvider) Models(ctx context.Context) ([]common.Model, error) {
	models := make([]common.Model, 0, len(miniMaxModels))
	for _, modelName := range miniMaxModels {
		models = append(models, common.Model{
			Name:          modelName,
			Provider:      common.ProviderName(p.connectContext.Name),
			ProviderAlias: p.connectContext.Alias,
		})
	}

	return models, nil
}

func (p *MiniMaxProvider) Ping(ctx context.Context) error {
	client := p.getClient(ctx)
	_, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: miniMaxModels[0],
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("ping"),
		},
		MaxCompletionTokens: openai.Int(1),
	})

	return err
}

func NewMiniMaxProvider(ctx context.Context, connectContext setting.AIProvider) Provider {
	if connectContext.Host == "" {
		connectContext.Host = miniMaxDefaultHost
	}

	return &MiniMaxProvider{
		OpenAIBaseProvider: NewOpenAIBaseProviderWithOptions(connectContext, OpenAIBaseProviderOptions{
			ChatRequestOptions: func(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions) []option.RequestOption {
				return []option.RequestOption{
					option.WithJSONSet("reasoning_split", true),
				}
			},
		}),
	}
}
