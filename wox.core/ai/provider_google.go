package ai

import (
	"context"
	"errors"
	"io"
	"wox/setting"
	"wox/util"

	"github.com/google/generative-ai-go/genai"
	"github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type GoogleProvider struct {
	connectContext setting.AIProvider
}

type GoogleProviderStream struct {
	stream        *genai.GenerateContentResponseIterator
	conversations []Conversation
	client        *genai.Client
}

func NewGoogleProvider(ctx context.Context, connectContext setting.AIProvider) Provider {
	return &GoogleProvider{connectContext: connectContext}
}

func (g *GoogleProvider) ChatStream(ctx context.Context, model Model, conversations []Conversation) (ChatStream, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(g.connectContext.ApiKey), option.WithHTTPClient(util.GetHTTPClient(ctx)))
	if err != nil {
		return nil, err
	}

	chatMessages, lastConversation := g.convertConversations(conversations)
	aiModel := client.GenerativeModel(model.Name)
	session := aiModel.StartChat()
	session.History = chatMessages
	stream := session.SendMessageStream(ctx, lastConversation.Parts...)
	return &GoogleProviderStream{conversations: conversations, stream: stream, client: client}, nil
}

func (g *GoogleProvider) Models(ctx context.Context) ([]Model, error) {
	return []Model{
		{
			Name:     "gemini-exp-1206",
			Provider: ProviderNameGoogle,
		},
		{
			Name:     "gemini-2.0-flash-exp",
			Provider: ProviderNameGoogle,
		},
	}, nil
}

func (g *GoogleProviderStream) Receive(ctx context.Context) (string, error) {
	response, err := g.stream.Next()
	if err != nil {
		// Close client when stream is done or error occurs
		if g.client != nil {
			g.client.Close()
			g.client = nil
		}

		// no more messages
		if errors.Is(err, iterator.Done) {
			return "", io.EOF
		}

		var v *apierror.APIError
		if errors.As(err, &v) {
			return "", v.Unwrap()
		}

		return "", err
	}
	if len(response.Candidates) == 0 {
		if g.client != nil {
			g.client.Close()
			g.client = nil
		}
		return "", io.EOF
	}

	for _, part := range response.Candidates[0].Content.Parts {
		if v, ok := part.(genai.Text); ok {
			return string(v), nil
		}
	}

	return "", errors.New("no text in response")
}

func (g *GoogleProvider) convertConversations(conversations []Conversation) (msgWithoutLast []*genai.Content, lastMsg *genai.Content) {
	var chatMessages []*genai.Content
	for _, conversation := range conversations {
		role := ""
		if conversation.Role == ConversationRoleUser {
			role = "user"
		}
		if conversation.Role == ConversationRoleAI {
			role = "model"
		}
		if role == "" {
			return nil, nil
		}

		chatMessages = append(chatMessages, &genai.Content{
			Parts: []genai.Part{
				genai.Text(conversation.Text),
			},
			Role: role,
		})
	}

	return chatMessages[:len(chatMessages)-1], chatMessages[len(chatMessages)-1]
}
