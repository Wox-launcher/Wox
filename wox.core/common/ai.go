package common

import "context"

type ConversationRole string
type ProviderName string
type ChatStreamDataType string

var (
	ConversationRoleUser ConversationRole = "user"
	ConversationRoleAI   ConversationRole = "ai"
)

var (
	ProviderNameOpenAI ProviderName = "openai"
	ProviderNameGoogle ProviderName = "google"
	ProviderNameOllama ProviderName = "ollama"
	ProviderNameGroq   ProviderName = "groq"
)

const (
	ChatStreamTypeStreaming ChatStreamDataType = "streaming"
	ChatStreamTypeFinished  ChatStreamDataType = "finished"
	ChatStreamTypeError     ChatStreamDataType = "error"
)

type ChatStreamFunc func(t ChatStreamDataType, data string)

type Conversation struct {
	Id        string
	Role      ConversationRole
	Text      string
	Images    []WoxImage
	Timestamp int64
}

type Model struct {
	Name     string
	Provider ProviderName
}

type AIChatData struct {
	Id            string
	Title         string
	Conversations []Conversation
	Model         Model

	CreatedAt int64
	UpdatedAt int64
}

type AIChater interface {
	Chat(ctx context.Context, aiChatData AIChatData)
}

var EmptyChatOptions = ChatOptions{}

type ChatOptions struct {
	Tools []Tool
}

type Tool struct {
	Name        string
	Description string
}
