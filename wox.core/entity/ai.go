package entity

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
	Role      ConversationRole
	Text      string
	Images    []WoxImage
	Timestamp int64
}

type Model struct {
	Name     string
	Provider ProviderName
}
