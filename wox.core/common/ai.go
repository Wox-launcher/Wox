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
	ChatStreamTypeToolCall  ChatStreamDataType = "tool_call" // json string of common.ToolCallInfo
)

const (
	ToolCallStatusStreaming ToolCallStatus = "streaming" // tool call is streaming, after streaming finished, tool call will be pending to be running
	ToolCallStatusPending   ToolCallStatus = "pending"
	ToolCallStatusRunning   ToolCallStatus = "running"
	ToolCallStatusSucceeded ToolCallStatus = "succeeded"
	ToolCallStatusFailed    ToolCallStatus = "failed"
)

type ChatStreamFunc func(result ChatStreamData)

type ChatStreamData struct {
	Type     ChatStreamDataType
	Data     string
	ToolCall ToolCallInfo // only available when type is common.ChatStreamTypeToolCall
}

type ToolCallInfo struct {
	Id        string
	Name      string
	Arguments map[string]any
	Status    ToolCallStatus

	Delta          string // when toolcall is streaming, we will put the delta content here
	Response       string
	StartTimestamp int64
	EndTimestamp   int64
}

type ToolCallStatus string

type Conversation struct {
	Id           string
	Role         ConversationRole
	Text         string
	Images       []WoxImage
	ToolCallInfo ToolCallInfo
	Timestamp    int64
}

type AIProviderInfo struct {
	Name ProviderName
	Icon WoxImage
}

type Model struct {
	Name     string
	Provider ProviderName
}

type AIAgent struct {
	Id     string
	Name   string
	Prompt string
	Model  Model
	Tools  []string
}

type AIChatData struct {
	Id            string
	Title         string
	Conversations []Conversation
	Model         Model
	Tools         []string
	AgentId       string

	CreatedAt int64
	UpdatedAt int64
}

type AIChater interface {
	Chat(ctx context.Context, aiChatData AIChatData, chatLoopCount int)
	GetAllTools(ctx context.Context) []MCPTool
	GetAllAgents(ctx context.Context) []AIAgent
	IsAutoFocusToChatInputWhenOpenWithQueryHotkey(ctx context.Context) bool
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
