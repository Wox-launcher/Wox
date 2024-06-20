package ai

import (
	"context"
	"errors"
	"fmt"
	"github.com/djherbis/buffer"
	"github.com/djherbis/nio/v3"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/schema"
	"io"
	"wox/setting"
	"wox/util"
)

type GroqProvider struct {
	connectContext setting.AIProvider
	client         *openai.LLM
}

type GroqProviderStream struct {
	conversations []Conversation
	reader        io.Reader
}

func NewGroqProvider(ctx context.Context, connectContext setting.AIProvider) Provider {
	return &GroqProvider{connectContext: connectContext}
}

func (g *GroqProvider) Close(ctx context.Context) error {
	return nil
}

func (g *GroqProvider) ChatStream(ctx context.Context, model Model, conversations []Conversation) (ChatStream, error) {
	client, clientErr := openai.New(openai.WithModel(model.Name), openai.WithBaseURL("https://api.groq.com/openai/v1"), openai.WithToken(g.connectContext.ApiKey))
	if clientErr != nil {
		return nil, clientErr
	}

	buf := buffer.New(4 * 1024) // 4KB In memory Buffer
	r, w := nio.Pipe(buf)
	util.Go(ctx, "Groq chat stream", func() {
		_, err := client.GenerateContent(ctx, g.convertConversations(conversations), llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			w.Write(chunk)
			return nil
		}))
		if err != nil {
			w.CloseWithError(err)
		} else {
			w.Close()
		}
	})

	return &GroqProviderStream{conversations: conversations, reader: r}, nil
}

func (g *GroqProvider) Models(ctx context.Context) (models []Model, err error) {
	return []Model{
		{
			Name:     "llama3-8b-8192",
			Provider: ProviderNameGroq,
		},
		{
			Name:     "llama3-70b-8192",
			Provider: ProviderNameGroq,
		},
		{
			Name:     "mixtral-8x7b-32768",
			Provider: ProviderNameGroq,
		},
		{
			Name:     "gemma-7b-it",
			Provider: ProviderNameGroq,
		},
	}, nil
}

func (g *GroqProvider) convertConversations(conversations []Conversation) (chatMessages []llms.MessageContent) {
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
