package common

import (
	"context"
	"fmt"

	"github.com/tmc/langchaingo/jsonschema"
)

type ConversationRole string
type ProviderName string
type ChatStreamDataType string

type AIChatMCPServerType string

const (
	AIChatMCPServerTypeSTDIO AIChatMCPServerType = "stdio"
	AIChatMCPServerTypeSSE   AIChatMCPServerType = "sse"
)

var (
	ConversationRoleSystem    ConversationRole = "system"
	ConversationRoleUser      ConversationRole = "user"
	ConversationRoleAssistant ConversationRole = "assistant"
	ConversationRoleTool      ConversationRole = "tool"
)

const (
	ChatStreamTypeStreaming ChatStreamDataType = "streaming"
	ChatStreamTypeFinished  ChatStreamDataType = "finished"
	ChatStreamTypeError     ChatStreamDataType = "error"
)

type ChatStreamFunc func(t ChatStreamDataType, data string)

type AIProviderInfo struct {
	Name ProviderName
	Icon WoxImage
}

type Conversation struct {
	Id         string
	Role       ConversationRole
	Text       string
	Images     []WoxImage
	ToolCallID string
	Timestamp  int64
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

	// Selected tools list, not persisted
	SelectedTools []string `json:"omitempty"`
}

type AIChater interface {
	Chat(ctx context.Context, aiChatData AIChatData)
	GetAllTools(ctx context.Context) []MCPTool
}

var EmptyChatOptions = ChatOptions{}

type ChatOptions struct {
	Tools []MCPTool
}

type MCPTool struct {
	Name        string
	Description string
	Parameters  jsonschema.Definition
	Callback    func(ctx context.Context, args map[string]any) (Conversation, error)

	ServerConfig *AIChatMCPServerConfig
}

func (t *MCPTool) Key() string {
	return fmt.Sprintf("%s:%s", t.ServerConfig.Name, t.Name)
}

type AIChatMCPServerConfig struct {
	Name     string
	Type     AIChatMCPServerType
	Disabled bool

	// for stdio server
	Command              string
	EnvironmentVariables []string //key=value

	// for sse server
	Url string
}
