package chatgpt

import (
	"context"
	"errors"
	"github.com/google/generative-ai-go/genai"
	"github.com/googleapis/gax-go/v2/apierror"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"io"
)

type GoogleClient struct {
	client *genai.Client
}

type GoogleClientStream struct {
	stream        *genai.GenerateContentResponseIterator
	conversations []Conversation
}

func NewGoogleClient(ctx context.Context, apiKey string) (Client, error) {
	client, newClientErr := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if newClientErr != nil {
		return nil, newClientErr
	}
	return &GoogleClient{client: client}, nil
}

func (c *GoogleClient) ChatStream(ctx context.Context, model chatgptModel, conversations []Conversation) (ClientChatStream, error) {
	chatMessages, lastConversation := c.convertConversations(conversations)
	aiModel := c.client.GenerativeModel(model.Name)
	session := aiModel.StartChat()
	session.History = chatMessages
	stream := session.SendMessageStream(ctx, lastConversation.Parts...)
	return &GoogleClientStream{conversations: conversations, stream: stream}, nil
}

func (c *GoogleClient) Chat(ctx context.Context, model chatgptModel, conversations []Conversation) (string, error) {
	chatMessages, lastConversation := c.convertConversations(conversations)
	aiModel := c.client.GenerativeModel(model.Name)
	session := aiModel.StartChat()
	session.History = chatMessages
	response, sendErr := session.SendMessage(ctx, lastConversation.Parts...)
	if sendErr != nil {
		return "", sendErr
	}

	for _, part := range response.Candidates[0].Content.Parts {
		if v, ok := part.(genai.Text); ok {
			return string(v), nil
		}
	}

	return "", errors.New("no text in response")
}

func (s *GoogleClientStream) Receive() (string, error) {
	response, err := s.stream.Next()
	if err != nil {
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
		return "", io.EOF
	}

	for _, part := range response.Candidates[0].Content.Parts {
		if v, ok := part.(genai.Text); ok {
			return string(v), nil
		}
	}

	return "", errors.New("no text in response")
}

func (s *GoogleClientStream) Close() {
	// no-op
}

func (c *GoogleClient) convertConversations(conversations []Conversation) (msgWithoutLast []*genai.Content, lastMsg *genai.Content) {
	var chatMessages []*genai.Content
	for _, conversation := range conversations {
		role := ""
		if conversation.Role == ConversationRoleUser {
			role = "user"
		}
		if conversation.Role == ConversationRoleSystem {
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
