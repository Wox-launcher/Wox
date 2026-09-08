package ai

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"wox/common"
	"wox/setting"
	"wox/util"

	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

func init() { providerFactories["grok-cli"] = NewGrokCLIClient }

type GrokCLIProvider struct{ *CLIBaseProvider }

// NewGrokCLIClient binds the installed command to its own protocol adapter.
func NewGrokCLIClient(_ context.Context, config setting.AIProvider) Provider {
	p := &GrokCLIProvider{}
	p.CLIBaseProvider = &CLIBaseProvider{config: config, provider: p}
	return p
}

// modelNames reads the installed command's authenticated catalog.
func (p *GrokCLIProvider) modelNames(ctx context.Context, workspace string) ([]string, error) {
	var names []string

	output, err := p.output(ctx, workspace, []string{"--no-auto-update", "models"})
	if err != nil {
		return nil, err
	}
	if strings.Contains(output, "not authenticated") {
		return nil, fmt.Errorf("sign in with grok login")
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && (fields[0] == "*" || fields[0] == "-") {
			names = append(names, fields[1])
		}
	}
	return names, nil
}

// turnCommand configures only this invocation, preserving the user's CLI settings.
func (p *GrokCLIProvider) turnCommand(workspace, model, effort, bridgeURL string, maxTurns int) ([]string, []string, error) {
	if maxTurns <= 0 {
		maxTurns = 25
	}
	limit := fmt.Sprint(maxTurns)
	enabledTools := ""
	if bridgeURL != "" {
		enabledTools = "search_tool,use_tool"
	}

	if bridgeURL != "" {
		if err := os.Mkdir(filepath.Join(workspace, ".grok"), 0700); err != nil {
			return nil, nil, err
		}
		config := fmt.Sprintf("[mcp_servers.wox]\nurl = %q\n", bridgeURL)
		if err := os.WriteFile(filepath.Join(workspace, ".grok", "config.toml"), []byte(config), 0600); err != nil {
			return nil, nil, err
		}
	}
	args := []string{"--no-auto-update", "--prompt-file", filepath.Join(workspace, "prompt.txt"), "--model", model, "--output-format", "streaming-json", "--cwd", workspace, "--tools", enabledTools, "--permission-mode", "dontAsk", "--no-plan", "--no-subagents", "--no-memory", "--disable-web-search", "--max-turns", limit, "--system-prompt-override", installedInstructions(bridgeURL), "--storage-mode", "local"}
	if bridgeURL != "" {
		args = append(args, "--allow", "MCPTool(wox__*)")
	}
	if effort != "" {
		args = append(args, "--effort", effort)
	}
	// The cwd is a fresh Wox-owned directory. Trust it for this process only,
	// rather than adding every temporary request directory to the user's trust store.
	return args, []string{"GROK_FOLDER_TRUST=0", "GROK_CLAUDE_MCPS_ENABLED=false", "GROK_CURSOR_MCPS_ENABLED=false"}, nil
}

// decodeGrokLine consumes model output; tool events come from the Wox bridge.
func decodeGrokLine(line []byte) (installedEvent, error) {
	if !gjson.ValidBytes(line) {
		return installedEvent{}, fmt.Errorf("Grok returned invalid JSON")
	}
	value := gjson.ParseBytes(line)
	event := installedEvent{}

	switch value.Get("type").String() {
	case "text":
		event.text = value.Get("data").String()
	case "thought":
		event.reasoning = value.Get("data").String()
	case "end":
		// An ACP stream can end because it hit a limit or was cancelled.
		// A successful process exit alone does not mean the agent answered.
		if reason := value.Get("stopReason").String(); reason != "end_turn" {
			return event, fmt.Errorf("Grok stopped without completing its turn (stopReason=%q)", reason)
		}
		event.done = true
	case "error", "max_turns_reached":
		return event, fmt.Errorf("Grok: %s %s", value.Get("type"), value.Get("message"))
	}
	return event, nil
}

// runCLITurn scopes the command's configuration and session to one Wox request.
func (p *GrokCLIProvider) runCLITurn(ctx context.Context, workspace string, model common.Model, conversations []common.Conversation, effort, bridgeURL string, maxTurns int, emit func(installedEvent)) error {
	args, env, err := p.turnCommand(workspace, model.Name, effort, bridgeURL, maxTurns)
	if err != nil {
		return err
	}
	prompt := installedPrompt(conversations, bridgeURL)
	sessionID := uuid.NewString()
	args = append(args, "--session-id", sessionID)
	if err := os.WriteFile(filepath.Join(workspace, "prompt.txt"), []byte(prompt), 0600); err != nil {
		return err
	}
	defer p.removeSession(ctx, workspace, []string{"--no-auto-update", "sessions", "delete", sessionID}, env)
	prompt = ""
	return p.runJSONTurn(ctx, workspace, args, env, prompt, emit, func(line []byte) (installedEvent, error) {
		value := gjson.ParseBytes(line)
		if value.Get("type").String() == "end" {
			util.GetLogger().Info(ctx, fmt.Sprintf("AI: CLI provider=grok stage=end stopReason=%.80s", value.Get("stopReason").String()))
		}
		// CLI discovery/dispatch failures never reach the Wox bridge, so retain
		// their bounded diagnostics without logging every text delta or tool input.
		if value.Get("type").String() == "tool_call_update" && value.Get("status").String() == "failed" {
			util.GetLogger().Warn(ctx, fmt.Sprintf("AI: CLI provider=grok toolCallId=%.120s failed output=%.512s", value.Get("toolCallId").String(), value.Get("rawOutput").String()))
		}
		return decodeGrokLine(line)
	})
}
