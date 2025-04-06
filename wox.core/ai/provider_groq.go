package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"wox/common"
	"wox/setting"
	"wox/util"

	"github.com/djherbis/buffer"
	"github.com/djherbis/nio/v3"
	"github.com/tidwall/gjson"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

const groqBaseUrl = "https://api.groq.com/openai/v1"

type GroqProvider struct {
	connectContext setting.AIProvider
}

type GroqProviderStream struct {
	conversations []common.Conversation
	reader        io.Reader
}

func NewGroqProvider(ctx context.Context, connectContext setting.AIProvider) Provider {
	return &GroqProvider{connectContext: connectContext}
}

func (g *GroqProvider) ChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions) (ChatStream, error) {
	client, clientErr := openai.New(
		openai.WithModel(model.Name),
		openai.WithBaseURL(groqBaseUrl),
		openai.WithToken(g.connectContext.ApiKey),
		openai.WithHTTPClient(util.GetHTTPClient(ctx)),
	)
	if clientErr != nil {
		return nil, clientErr
	}

	buf := buffer.New(4 * 1024) // 4KB In memory Buffer
	r, w := nio.Pipe(buf)
	util.Go(ctx, "Groq chat stream", func() {
		response, err := client.GenerateContent(ctx, g.convertConversations(conversations), llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			w.Write(chunk)
			return nil
		}), llms.WithTools(g.convertTools(options.Tools)), llms.WithToolChoice("auto"))
		if err != nil {
			w.CloseWithError(err)
		} else {
			fc := response.Choices[0].FuncCall
			if fc != nil {
				toolCall := fc.Name
				for _, tool := range options.Tools {
					if tool.Name == toolCall {
						util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf("Groq: Tool call: %s", toolCall))
						var params map[string]any
						json.Unmarshal([]byte(fc.Arguments), &params)
						toolResp, err := tool.Callback(ctx, params)
						if err != nil {
							util.GetLogger().Error(util.NewTraceContext(), fmt.Sprintf("Groq: Tool call: %s, error: %s", toolCall, err))
						} else {
							// replace the last user conversation with the tool call result
							lastUserConversation := conversations[len(conversations)-1]
							conversations[len(conversations)-1].Text = fmt.Sprintf("%s, here is the tool call result: %s, please use this result to answer the question", lastUserConversation.Text, toolResp.Text)

							_, err := client.GenerateContent(ctx, g.convertConversations(conversations), llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
								w.Write(chunk)
								return nil
							}))
							if err != nil {
								util.GetLogger().Error(util.NewTraceContext(), fmt.Sprintf("Groq: Post tool call: %s, error: %s", toolCall, err))
							}
						}
					}
				}
			}

			w.Close()
		}
	})

	return &GroqProviderStream{conversations: conversations, reader: r}, nil
}

func (g *GroqProvider) convertTools(tools []common.MCPTool) []llms.Tool {
	/*
		{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        "getCurrentWeather",
				Description: "Get the current weather in a given location",
				Parameters: jsonschema.Definition{
					Type: jsonschema.Object,
					Properties: map[string]jsonschema.Definition{
						"rationale": {
							Type:        jsonschema.String,
							Description: "The rationale for choosing this function call with these parameters",
						},
						"location": {
							Type:        jsonschema.String,
							Description: "The city and state, e.g. San Francisco, CA",
						},
						"unit": {
							Type: jsonschema.String,
							Enum: []string{"celsius", "fahrenheit"},
						},
					},
					Required: []string{"rationale", "location"},
				},
			},
		}
	*/
	convertedTools := make([]llms.Tool, len(tools))
	for i, tool := range tools {
		convertedTools[i] = llms.Tool{
			Type: "function",
			Function: &llms.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		}
	}
	return convertedTools
}

func (g *GroqProvider) Models(ctx context.Context) (models []common.Model, err error) {
	body, err := util.HttpGetWithHeaders(ctx, groqBaseUrl+"/models", map[string]string{
		"Authorization": "Bearer " + g.connectContext.ApiKey,
	})
	if err != nil {
		return nil, err
	}

	// response example
	//{
	//   "object": "list",
	//   "data": [
	//     {
	//       "id": "llama3-groq-70b-8192-tool-use-preview",
	//       "object": "model",
	//       "created": 1693721698,
	//       "owned_by": "Groq",
	//       "active": true,
	//       "context_window": 8192,
	//       "public_apps": null
	//     },
	//     {
	//       "id": "gemma2-9b-it",
	//       "object": "model",
	//       "created": 1693721698,
	//       "owned_by": "Google",
	//       "active": true,
	//       "context_window": 8192,
	//       "public_apps": null
	//     }
	//   ]
	// }

	// only return active models
	gjson.Get(string(body), "data").ForEach(func(key, value gjson.Result) bool {
		if value.Get("active").Bool() {
			models = append(models, common.Model{
				Name:     value.Get("id").String(),
				Provider: common.ProviderNameGroq,
			})
		}
		return true
	})

	return models, nil
}

func (g *GroqProvider) Ping(ctx context.Context) error {
	_, err := util.HttpGetWithHeaders(ctx, groqBaseUrl+"/models", map[string]string{
		"Authorization": "Bearer " + g.connectContext.ApiKey,
	})
	return err
}

func (g *GroqProvider) convertConversations(conversations []common.Conversation) (chatMessages []llms.MessageContent) {
	for _, conversation := range conversations {
		if conversation.Role == common.ConversationRoleUser {
			chatMessages = append(chatMessages, llms.TextParts(llms.ChatMessageTypeHuman, conversation.Text))
		}
		if conversation.Role == common.ConversationRoleAI {
			chatMessages = append(chatMessages, llms.TextParts(llms.ChatMessageTypeAI, conversation.Text))
		}
	}

	return chatMessages
}

func (s *GroqProviderStream) Receive(ctx context.Context) (string, error) {
	buf := make([]byte, 2048)
	n, err := s.reader.Read(buf)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", io.EOF
		}
		return "", err
	}

	resp := string(buf[:n])
	util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf("Groq: Send response: %s", resp))
	return resp, nil
}
