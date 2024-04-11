package chatgpt

import (
	"context"
	"errors"
)

type OllamaProvider struct {
	url string
}

type OllamaProviderStream struct {
	conversations []Conversation
}

func NewOllamaProvider(ctx context.Context, url string) Provider {
	return &OllamaProvider{url: url}
}

func (o *OllamaProvider) Connect(ctx context.Context) error {
	return nil
}

func (o *OllamaProvider) Close(ctx context.Context) error {
	return nil
}

func (o *OllamaProvider) ChatStream(ctx context.Context, model chatgptModel, conversations []Conversation) (ProviderChatStream, error) {
	return nil, errors.New("not implemented")
}

func (o *OllamaProvider) Chat(ctx context.Context, model chatgptModel, conversations []Conversation) (string, error) {
	return "", errors.New("not implemented")
}

func (o *OllamaProvider) Models(ctx context.Context) ([]chatgptModel, error) {
	return []chatgptModel{
		{
			DisplayName: "ollama",
			Name:        "ollama",
			Provider:    chatgptModelProviderNameOllama,
		},
	}, nil
}

func (s *OllamaProviderStream) Receive() (string, error) {
	return "", errors.New("no text in response")
}

func (s *OllamaProviderStream) Close() {
	// no-op
}
