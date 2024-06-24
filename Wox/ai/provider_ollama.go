package ai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/djherbis/buffer"
	"github.com/djherbis/nio/v3"
	"github.com/tidwall/gjson"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/ollama"
	"github.com/tmc/langchaingo/schema"
	"image/png"
	"io"
	"wox/setting"
	"wox/util"
)

type OllamaProvider struct {
	connectContext setting.AIProvider
	client         *ollama.LLM
}

type OllamaProviderStream struct {
	conversations []Conversation
	reader        io.Reader
}

func NewOllamaProvider(ctx context.Context, connectContext setting.AIProvider) Provider {
	return &OllamaProvider{connectContext: connectContext}
}

func (o *OllamaProvider) Close(ctx context.Context) error {
	return nil
}

func (o *OllamaProvider) ChatStream(ctx context.Context, model Model, conversations []Conversation) (ChatStream, error) {
	client, clientErr := ollama.New(ollama.WithServerURL(o.connectContext.Host), ollama.WithModel(model.Name))
	if clientErr != nil {
		return nil, clientErr
	}

	buf := buffer.New(4 * 1024) // 4KB In memory Buffer
	r, w := nio.Pipe(buf)
	util.Go(ctx, "ollama chat stream", func() {
		_, err := client.GenerateContent(ctx, o.convertConversations(conversations), llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			w.Write(chunk)
			return nil
		}))
		if err != nil {
			w.CloseWithError(err)
		} else {
			w.Close()
		}
	})

	return &OllamaProviderStream{conversations: conversations, reader: r}, nil
}

func (o *OllamaProvider) Models(ctx context.Context) (models []Model, err error) {
	body, err := util.HttpGet(ctx, o.connectContext.Host+"/api/tags")
	if err != nil {
		return nil, err
	}

	gjson.Get(string(body), "models.#.name").ForEach(func(key, value gjson.Result) bool {
		models = append(models, Model{
			Name:     value.String(),
			Provider: ProviderNameOllama,
		})
		return true
	})

	return models, nil
}

func (o *OllamaProvider) convertConversations(conversations []Conversation) (chatMessages []llms.MessageContent) {
	for _, conversation := range conversations {
		var msg llms.MessageContent
		if conversation.Role == ConversationRoleUser {
			msg = llms.TextParts(schema.ChatMessageTypeHuman, conversation.Text)
		}
		if conversation.Role == ConversationRoleSystem {
			msg = llms.TextParts(schema.ChatMessageTypeSystem, conversation.Text)
		}

		for _, image := range conversation.Images {
			buf := new(bytes.Buffer)
			err := png.Encode(buf, image)
			if err != nil {
				util.GetLogger().Error(util.NewTraceContext(), err.Error())
				continue
			}
			msg.Parts = append(msg.Parts, llms.BinaryPart("image/png", buf.Bytes()))
		}

		chatMessages = append(chatMessages, msg)
	}

	return chatMessages
}

func (s *OllamaProviderStream) Receive(ctx context.Context) (string, error) {
	buf := make([]byte, 2048)
	n, err := s.reader.Read(buf)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", io.EOF
		}
		return "", err
	}

	resp := string(buf[:n])
	util.GetLogger().Debug(util.NewTraceContext(), fmt.Sprintf("OLLAMA: Send response: %s", resp))
	return resp, nil
}
