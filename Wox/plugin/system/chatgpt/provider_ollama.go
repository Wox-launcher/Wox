package chatgpt

import (
	"context"
	"errors"
	"github.com/tidwall/gjson"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/schema"
	"io"
	"wox/util"
)

type OllamaProvider struct {
	connectContext chatgptProviderConnectContext
	client         *ollama.LLM
}

type OllamaProviderStream struct {
	conversations []Conversation
	reader        io.Reader
}

func NewOllamaProvider(ctx context.Context, connectContext chatgptProviderConnectContext) Provider {
	return &OllamaProvider{connectContext: connectContext}
}

func (o *OllamaProvider) Connect(ctx context.Context) error {
	client, clientErr := ollama.New(ollama.WithServerURL(o.connectContext.Host))
	if clientErr != nil {
		return clientErr
	}

	o.client = client
	return nil
}

func (o *OllamaProvider) Close(ctx context.Context) error {
	return nil
}

func (o *OllamaProvider) ChatStream(ctx context.Context, model chatgptModel, conversations []Conversation) (ProviderChatStream, error) {
	client, clientErr := ollama.New(ollama.WithServerURL(o.connectContext.Host), ollama.WithModel(model.Name))
	if clientErr != nil {
		return nil, clientErr
	}

	reader, writer := io.Pipe()

	util.Go(ctx, "ollama chat stream", func() {
		_, err := client.GenerateContent(ctx, o.convertConversations(conversations), llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			writer.Write(chunk)
			return nil
		}))
		if err != nil {
			writer.CloseWithError(err)
		} else {
			writer.Close()
		}
	})

	return &OllamaProviderStream{conversations: conversations, reader: reader}, nil
}

func (o *OllamaProvider) Chat(ctx context.Context, model chatgptModel, conversations []Conversation) (string, error) {
	client, clientErr := ollama.New(ollama.WithServerURL(o.connectContext.Host), ollama.WithModel(model.Name))
	if clientErr != nil {
		return "", clientErr
	}

	response, responseErr := client.GenerateContent(ctx, o.convertConversations(conversations))
	if responseErr != nil {
		return "", responseErr
	}

	return response.Choices[0].Content, nil
}

func (o *OllamaProvider) Models(ctx context.Context) (models []chatgptModel, err error) {
	body, err := util.HttpGet(ctx, o.connectContext.Host+"/api/tags")
	if err != nil {
		return nil, err
	}

	gjson.Get(string(body), "models.#.name").ForEach(func(key, value gjson.Result) bool {
		models = append(models, chatgptModel{
			DisplayName: value.String(),
			Name:        value.String(),
			Provider:    chatgptModelProviderNameOllama,
		})
		return true
	})

	return models, nil
}

func (o *OllamaProvider) convertConversations(conversations []Conversation) (chatMessages []llms.MessageContent) {
	for _, conversation := range conversations {
		if conversation.Role == ConversationRoleUser {
			chatMessages = append(chatMessages, llms.TextParts(schema.ChatMessageTypeHuman, conversation.Text))
		}
		if conversation.Role == ConversationRoleSystem {
			chatMessages = append(chatMessages, llms.TextParts(schema.ChatMessageTypeSystem, conversation.Text))
		}
	}

	return chatMessages
}

func (s *OllamaProviderStream) Receive() (string, error) {
	buf := make([]byte, 1024)
	n, err := s.reader.Read(buf)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", io.EOF
		}
		return "", err
	}

	return string(buf[:n]), nil
}

func (s *OllamaProviderStream) Close() {
	// no-op
}
