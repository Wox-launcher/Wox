package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"wox/common"
	"wox/setting"

	"github.com/tidwall/gjson"
)

func init() { providerFactories["claude-cli"] = NewClaudeCodeCLIClient }

type ClaudeCodeCLIProvider struct{ *CLIBaseProvider }

// NewClaudeCodeCLIClient binds the installed command to its own protocol adapter.
func NewClaudeCodeCLIClient(_ context.Context, config setting.AIProvider) Provider {
	p := &ClaudeCodeCLIProvider{}
	p.CLIBaseProvider = &CLIBaseProvider{config: config, provider: p, npmEntry: "@anthropic-ai/claude-code/cli.js"}
	return p
}

// modelNames reads the installed command's authenticated catalog.
func (p *ClaudeCodeCLIProvider) modelNames(ctx context.Context, workspace string) ([]string, error) {
	output, err := p.output(ctx, workspace, []string{"auth", "status", "--json"})
	if err != nil {
		return nil, err
	}
	if !gjson.Get(output, "loggedIn").Bool() && !gjson.Get(output, "authenticated").Bool() {
		return nil, fmt.Errorf("sign in with claude auth login")
	}
	return []string{"sonnet", "opus", "haiku"}, nil
}

// turnCommand configures only this invocation, preserving the user's CLI settings.
func (p *ClaudeCodeCLIProvider) turnCommand(workspace, model, effort, bridgeURL string, maxTurns int) ([]string, []string, error) {
	if maxTurns <= 0 {
		maxTurns = 25
	}
	limit := fmt.Sprint(maxTurns)

	servers := map[string]any{}
	if bridgeURL != "" {
		servers["wox"] = map[string]any{"type": "http", "url": bridgeURL}
	}
	config, _ := json.Marshal(map[string]any{"mcpServers": servers})
	args := []string{"-p", "--model", model, "--input-format", "text", "--output-format", "stream-json", "--verbose", "--include-partial-messages", "--no-session-persistence", "--disable-slash-commands", "--setting-sources", "", "--tools", "", "--strict-mcp-config", "--mcp-config", string(config), "--no-chrome", "--max-turns", limit, "--system-prompt", installedInstructions(bridgeURL)}
	if bridgeURL != "" {
		args = append(args, "--allowedTools", "mcp__wox__*")
	}
	if effort != "" {
		args = append(args, "--effort", effort)
	}
	return args, []string{"CLAUDE_CODE_SKIP_PROMPT_HISTORY=1", "ENABLE_CLAUDEAI_MCP_SERVERS=false"}, nil
}

// decodeClaudeCodeLine consumes model output; tool events come from the Wox bridge.
func decodeClaudeCodeLine(line []byte) (installedEvent, error) {
	if !gjson.ValidBytes(line) {
		return installedEvent{}, fmt.Errorf("ClaudeCode returned invalid JSON")
	}
	value := gjson.ParseBytes(line)
	event := installedEvent{}

	switch value.Get("type").String() {
	case "stream_event":
		delta := value.Get("event.delta")
		if delta.Get("type").String() == "text_delta" {
			event.text = delta.Get("text").String()
		}
		if delta.Get("type").String() == "thinking_delta" {
			event.reasoning = delta.Get("thinking").String()
		}
	case "result":
		if value.Get("is_error").Bool() {
			return event, fmt.Errorf("Claude: %s %s", value.Get("result"), value.Get("errors"))
		}
		event.done = true
	}
	return event, nil
}

// runCLITurn scopes the command's configuration and session to one Wox request.
func (p *ClaudeCodeCLIProvider) runCLITurn(ctx context.Context, workspace string, model common.Model, conversations []common.Conversation, effort, bridgeURL string, maxTurns int, emit func(installedEvent)) error {
	if effort != "" && (model.Name == "haiku" || !slices.Contains([]string{"low", "medium", "high", "xhigh", "max"}, effort)) {
		return fmt.Errorf("Claude %s does not support effort %s", model.Name, effort)
	}
	args, env, err := p.turnCommand(workspace, model.Name, effort, bridgeURL, maxTurns)
	if err != nil {
		return err
	}
	prompt := installedPrompt(conversations, bridgeURL)
	return p.runJSONTurn(ctx, workspace, args, env, prompt, emit, decodeClaudeCodeLine)
}
