package common

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tmc/langchaingo/jsonschema"
)

type ConversationRole string
type ProviderName string
type ChatStreamDataStatus string
type ChatThinkingMode string

type AIChatMCPServerType string

const (
	AIChatMCPServerTypeSTDIO          AIChatMCPServerType = "stdio"
	AIChatMCPServerTypeStreamableHTTP AIChatMCPServerType = "streamable-http"
)

const (
	ChatThinkingModeProviderDefault ChatThinkingMode = "provider_default"
	ChatThinkingModeThinking        ChatThinkingMode = "thinking"
	ChatThinkingModeNonThinking     ChatThinkingMode = "non_thinking"
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

// AISkillRef is the stable message-level pointer to a local SKILL.md bundle.
type AISkillRef struct {
	Id     string
	Name   string
	Path   string
	Source string
}

type Conversation struct {
	Id           string
	Role         ConversationRole
	Text         string
	Reasoning    string // Reasoning content from models that support reasoning (e.g., DeepSeek, OpenAI o1, qwen3)
	Images       []WoxImage
	SkillRefs    []AISkillRef
	ToolCallInfo ToolCallInfo
	Timestamp    int64
}

type AIProviderInfo struct {
	Name        ProviderName
	Icon        WoxImage
	DefaultHost string
}

type Model struct {
	Name          string
	Provider      ProviderName
	ProviderAlias string // optional, used to choose the correct provider config when there are multiple
}

func (m *Model) ProviderName() string {
	if m.ProviderAlias != "" {
		return m.ProviderAlias
	}

	return string(m.Provider)
}

type AIChatData struct {
	Id            string
	Title         string
	Conversations []Conversation
	Model         Model

	CreatedAt int64
	UpdatedAt int64
}

// AIChatPreviewData bootstraps the chat preview app with an active draft and the saved chat list.
type AIChatPreviewData struct {
	ActiveChat AIChatData
	Chats      []AIChatData
}

type AIChater interface {
	Chat(ctx context.Context, aiChatData AIChatData, chatLoopCount int)
	DeleteChat(ctx context.Context, chatId string) bool
	SummarizeChat(ctx context.Context, chatId string) bool
	GetAllTools(ctx context.Context) []MCPTool
	GetAllSkills(ctx context.Context) []Skill
	ReloadMCPServers(ctx context.Context, notifyUI bool)
	ReloadSkills(ctx context.Context) error
	GetDefaultModel(ctx context.Context) Model
}

var EmptyChatOptions = ChatOptions{}

type ChatOptions struct {
	Tools         []Tool
	ThinkingMode  ChatThinkingMode
	LoopPolicy    LoopPolicy
	ContextPolicy ContextPolicy
	// OnSummarize, when set, is invoked at the top of each loop iteration to
	// optionally summarize old conversations. Returns the (possibly shortened)
	// conversation list.
	OnSummarize func(ctx context.Context, conversations []Conversation, policy ContextPolicy) []Conversation
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

// ToolSource identifies where a tool was registered.
type ToolSource string

const (
	ToolSourceMCP     ToolSource = "mcp"
	ToolSourceBuiltin ToolSource = "builtin"
)

// Tool is the unified representation that the AI consumes for any callable tool.
// MCPTool is kept for backward compatibility; MCP tools are wrapped into Tool
// at the registry layer. Builtin tools use this type directly.
type Tool struct {
	Name         string
	Description  string
	Parameters   jsonschema.Definition
	Callback     func(ctx context.Context, args map[string]any) (ToolResult, error)
	Source       ToolSource
	ServerConfig *AIChatMCPServerConfig // nil for builtin tools
}

// ToolResult replaces the legacy (Conversation, error) tool callback return.
// Tools return plain text (and optional images); callers wrap this into a
// Conversation when needed.
type ToolResult struct {
	Text   string
	Images []WoxImage
}

// AIQuestionOption describes one selectable answer for the ask_user tool.
// Value is returned to the model; Title/SubTitle are UI presentation hints.
type AIQuestionOption struct {
	Value       string
	Title       string
	SubTitle    string
	Recommended bool
	Extra       map[string]string
}

// ToTool bridges an MCPTool into the unified Tool type. The MCP callback's
// Conversation result is unwrapped into ToolResult so callers can use a single
// callback shape regardless of tool source.
func (m *MCPTool) ToTool() Tool {
	return Tool{
		Name:        m.Name,
		Description: m.Description,
		Parameters:  m.Parameters,
		Callback: func(ctx context.Context, args map[string]any) (ToolResult, error) {
			conv, err := m.Callback(ctx, args)
			if err != nil {
				return ToolResult{}, err
			}
			return ToolResult{Text: conv.Text, Images: conv.Images}, nil
		},
		Source:       ToolSourceMCP,
		ServerConfig: m.ServerConfig,
	}
}

// LoopPolicy controls the tool-enabled chat loop in AIChatStream.
type LoopPolicy struct {
	MaxIterations  int           // 0 means default (25); -1 means unlimited
	RetryOnFailure bool          // when true, tool errors are fed back to the model instead of aborting
	MaxRetries     int           // per-iteration retry cap for a single failing tool call; 0 means default (3)
	Timeout        time.Duration // 0 means no per-loop timeout
}

// ContextPolicy controls when long conversations get summarized to avoid token overflow.
type ContextPolicy struct {
	MaxConversations int // threshold to trigger summarization; 0 disables
	SummarizeToCount int // target conversation count after summarization
	Enabled          bool
}

// Skill describes a discovered SKILL.md bundle that a model can reference.
type Skill struct {
	Id                     string
	Name                   string
	Description            string
	Path                   string
	ManifestPath           string
	Source                 string
	SourceName             string
	Builtin                bool
	ReadOnly               bool
	Error                  string
	Enabled                bool
	DisableModelInvocation bool

	// Deprecated: legacy manually configured skills used inline instructions.
	// Keep these fields for settings compatibility while discovered skills
	// become the runtime source of truth.
	Instructions string
	Tools        []string
	Templates    map[string]string
	Icon         WoxImage
}

type AIChatMCPServerConfig struct {
	Name     string
	Type     AIChatMCPServerType
	Disabled bool

	// for stdio server
	Command              string
	EnvironmentVariables []string //key=value

	// for streamable http server
	Url string
}
