package ai

import (
	"context"
	"io"
	"iter"
	"wox/setting"
	"wox/util"

	"google.golang.org/genai"
)

type GoogleProvider struct {
	connectContext setting.AIProvider
}

type GoogleProviderStream struct {
	stream        func() (*genai.GenerateContentResponse, error, bool)
	conversations []Conversation
	client        *genai.Client
}

func NewGoogleProvider(ctx context.Context, connectContext setting.AIProvider) Provider {
	return &GoogleProvider{connectContext: connectContext}
}

func (g *GoogleProvider) ChatStream(ctx context.Context, model Model, conversations []Conversation) (ChatStream, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:     g.connectContext.ApiKey,
		Backend:    genai.BackendGeminiAPI,
		HTTPClient: util.GetHTTPClient(ctx),
	})
	if err != nil {
		return nil, err
	}

	chatMessages := g.convertConversations(conversations)
	stream := client.Models.GenerateContentStream(ctx, model.Name, chatMessages, &genai.GenerateContentConfig{})
	next, _ := iter.Pull2(stream)
	return &GoogleProviderStream{conversations: conversations, stream: next, client: client}, nil
}

func (g *GoogleProvider) Models(ctx context.Context) ([]Model, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:     g.connectContext.ApiKey,
		Backend:    genai.BackendGeminiAPI,
		HTTPClient: util.GetHTTPClient(ctx),
	})
	if err != nil {
		return nil, err
	}

	models, err := client.Models.List(ctx, &genai.ListModelsConfig{})
	if err != nil {
		return nil, err
	}

	var googleModels []Model
	for _, model := range models.Items {
		googleModels = append(googleModels, Model{
			Name:     model.Name,
			Provider: ProviderNameGoogle,
		})
	}

	for {
		models, err := models.Next(ctx)
		if err != nil {
			break
		}

		for _, model := range models.Items {
			googleModels = append(googleModels, Model{
				Name:     model.Name,
				Provider: ProviderNameGoogle,
			})
		}
	}

	return googleModels, nil
}

func (g *GoogleProvider) Ping(ctx context.Context) error {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:     g.connectContext.ApiKey,
		Backend:    genai.BackendGeminiAPI,
		HTTPClient: util.GetHTTPClient(ctx),
	})
	if err != nil {
		return err
	}

	_, err = client.Models.List(ctx, &genai.ListModelsConfig{})
	return err
}

func (g *GoogleProviderStream) Receive(ctx context.Context) (string, error) {
	response, err, valid := g.stream()
	if err != nil {
		return "", err
	}
	if !valid {
		// finished
		return "", io.EOF
	}

	return response.Text(), nil
}

func (g *GoogleProvider) convertConversations(conversations []Conversation) (newConversations []*genai.Content) {
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
			return nil
		}

		chatMessages = append(chatMessages, &genai.Content{
			Parts: []*genai.Part{
				{
					Text: conversation.Text,
				},
			},
			Role: role,
		})
	}

	return chatMessages
}
