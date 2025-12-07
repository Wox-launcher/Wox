package common

import (
	"context"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/jsonschema"
)

type ConversationRole string
type ProviderName string
type ChatStreamDataStatus string

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
	ChatStreamStatusStreaming       ChatStreamDataStatus = "streaming"         // steaming data or tool call
	ChatStreamStatusStreamed        ChatStreamDataStatus = "streamed"          // all data and tool call streamed, if there is any tool call, it will be running_tool_call next, otherwise finished
	ChatStreamStatusRunningToolCall ChatStreamDataStatus = "running_tool_call" // running all tool calls, after all tool call finished, it will be finished
	ChatStreamStatusFinished        ChatStreamDataStatus = "finished"          // all data and tool call(if any) finished
	ChatStreamStatusError           ChatStreamDataStatus = "error"             // error occurred between sreaming or tool call
)

const (
	ToolCallStatusStreaming ToolCallStatus = "streaming" // tool call is streaming, after streaming finished, tool call will be pending to be running
	ToolCallStatusPending   ToolCallStatus = "pending"   // streaming finished, ready to run
	ToolCallStatusRunning   ToolCallStatus = "running"
	ToolCallStatusSucceeded ToolCallStatus = "succeeded"
	ToolCallStatusFailed    ToolCallStatus = "failed"
)

type ChatStreamFunc func(result ChatStreamData)

type ChatStreamData struct {
	Status ChatStreamDataStatus
	// Aggregated data, E.g. Data is streamed by 3 chunks, then Data1 = chunk1, Data2 = chunk1 + chunk2, Data3 = chunk1 + chunk2 + chunk3
	Data string
	// Reasoning content from models that support reasoning (e.g., DeepSeek, OpenAI o1). Separate from Data for clean processing.
	Reasoning string
	ToolCalls []ToolCallInfo
}

func (c *ChatStreamData) IsNotFinished() bool {
	return c.Status == ChatStreamStatusStreaming || c.Status == ChatStreamStatusStreamed || c.Status == ChatStreamStatusRunningToolCall
}

func (c *ChatStreamData) IsAllToolCallsSucceeded() bool {
	if c.Status != ChatStreamStatusFinished {
		return false
	}
	if len(c.ToolCalls) == 0 {
		return false
	}

	for _, toolCall := range c.ToolCalls {
		if toolCall.Status != ToolCallStatusSucceeded {
			return false
		}
	}

	return true
}

func (c *ChatStreamData) ToMarkdown() string {
	content := c.Data
	thinking := c.Reasoning

	if thinking == "" {
		return content
	}

	// everyline in thinking should be prefixed with "> "
	thinkingLines := strings.Split(thinking, "\n")
	for i, line := range thinkingLines {
		thinkingLines[i] = "> " + line
	}
	thinking = strings.Join(thinkingLines, "\n")

	return thinking + "\n\n" + content
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
	Reasoning    string // Reasoning content from models that support reasoning (e.g., DeepSeek, OpenAI o1, qwen3)
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
	Name   string
	Prompt string
	Model  Model
	Tools  []string
	Icon   WoxImage
}

type AIChatData struct {
	Id            string
	Title         string
	Conversations []Conversation
	Model         Model
	Tools         []string
	AgentName     string

	CreatedAt int64
	UpdatedAt int64
}

type AIChater interface {
	Chat(ctx context.Context, aiChatData AIChatData, chatLoopCount int)
	GetAllTools(ctx context.Context) []MCPTool
	GetAllAgents(ctx context.Context) []AIAgent
	GetDefaultModel(ctx context.Context) Model
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
