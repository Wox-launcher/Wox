package plugin

import (
	"context"
	"testing"
	"wox/ai"
	"wox/common"
	"wox/util"
)

type installedLoopTestProvider struct{ calls int }

func (p *installedLoopTestProvider) GetIcon() common.WoxImage                       { return common.WoxImage{} }
func (p *installedLoopTestProvider) GetDefaultHost() string                         { return "" }
func (p *installedLoopTestProvider) Models(context.Context) ([]common.Model, error) { return nil, nil }
func (p *installedLoopTestProvider) Ping(context.Context) error                     { return nil }

// ChatStream emulates an agent that has already executed a tool through the supplied Wox executor.
func (p *installedLoopTestProvider) ChatStream(ctx context.Context, _ common.Model, _ []common.Conversation, options common.ChatOptions) (ai.ChatStream, error) {
	tool := common.Tool{Name: "echo", Source: common.ToolSourceBuiltin, Callback: func(context.Context, map[string]any) (common.ToolResult, error) {
		p.calls++
		return common.ToolResult{Text: "done"}, nil
	}}
	result := options.ExecuteTool(ctx, common.AgentToolExecutionOption{Tool: tool, Call: common.ToolCallInfo{Id: "one", Name: "echo"}, OnUpdate: func(common.ToolCallInfo) {}})
	return installedLoopTestStream{result.Call}, nil
}

type installedLoopTestStream struct{ call common.ToolCallInfo }

func (s installedLoopTestStream) Receive(context.Context) (common.ChatStreamData, error) {
	return common.ChatStreamData{Status: common.ChatStreamStatusStreamed, Data: "done", ToolCalls: []common.ToolCallInfo{s.call}}, nil
}

// TestInstalledAgentDoesNotExecuteToolsTwice exercises the core loop, not just the MCP bridge.
func TestInstalledAgentDoesNotExecuteToolsTwice(t *testing.T) {
	provider := &installedLoopTestProvider{}
	api := &APIImpl{toolCallStartTimeMap: util.NewHashMap[string, int64]()}
	finished := false
	api.runChatLoop(context.Background(), common.Model{Provider: "codex-cli"}, nil, common.ChatOptions{}, common.LoopPolicy{MaxIterations: 2}, provider, func(data common.ChatStreamData) {
		if data.Status == common.ChatStreamStatusFinished {
			finished = true
		}
		if data.Status == common.ChatStreamStatusError {
			t.Fatal(data.Data)
		}
	})
	if !finished || provider.calls != 1 {
		t.Fatalf("finished=%v tool executions=%d", finished, provider.calls)
	}
}
