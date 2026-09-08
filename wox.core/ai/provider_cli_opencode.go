package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"wox/common"
	"wox/setting"

	"github.com/tidwall/gjson"
)

func init() { providerFactories["opencode-cli"] = NewOpenCodeCLIClient }

type OpenCodeCLIProvider struct{ *CLIBaseProvider }

// NewOpenCodeCLIClient binds the installed command to its own protocol adapter.
func NewOpenCodeCLIClient(_ context.Context, config setting.AIProvider) Provider {
	p := &OpenCodeCLIProvider{}
	p.CLIBaseProvider = &CLIBaseProvider{config: config, provider: p, npmEntry: "opencode-ai/bin/opencode"}
	return p
}

// modelNames reads the installed command's authenticated catalog.
func (p *OpenCodeCLIProvider) modelNames(ctx context.Context, workspace string) ([]string, error) {
	var names []string

	output, err := p.output(ctx, workspace, []string{"models", "--pure", "--verbose"})
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == line && strings.Contains(line, "/") && !strings.ContainsAny(line, " {}\t\x1b") {
			names = append(names, line)
		}
	}
	return names, nil
}

// turnCommand configures only this invocation, preserving the user's CLI settings.
func (p *OpenCodeCLIProvider) turnCommand(workspace, model, effort, bridgeURL string, maxTurns int) ([]string, []string, error) {
	config := map[string]any{"permission": map[string]string{"*": "deny", "wox_*": "allow"}, "share": "disabled", "mcp": map[string]any{}}
	if bridgeURL != "" {
		config["mcp"] = map[string]any{"wox": map[string]any{"type": "remote", "url": bridgeURL, "enabled": true, "oauth": false}}
	}
	data, _ := json.Marshal(config)
	args := []string{"run", "--pure", "--format", "json", "--model", model, "--dir", workspace, "--title", "Wox"}
	if effort != "" {
		args = append(args, "--variant", effort)
	}
	return args, []string{"OPENCODE_CONFIG_CONTENT=" + string(data), "OPENCODE_AUTO_SHARE=false", "OPENCODE_DISABLE_AUTOUPDATE=true"}, nil
}

// decodeOpenCodeLine consumes model output; tool events come from the Wox bridge.
func decodeOpenCodeLine(line []byte) (installedEvent, error) {
	if !gjson.ValidBytes(line) {
		return installedEvent{}, fmt.Errorf("OpenCode returned invalid JSON")
	}
	value := gjson.ParseBytes(line)
	event := installedEvent{}

	switch value.Get("type").String() {
	case "text":
		event.text = value.Get("part.text").String()
	case "reasoning":
		event.reasoning = value.Get("part.text").String()
	case "step_finish":
		event.done = value.Get("part.reason").String() == "stop"
	case "error":
		return event, fmt.Errorf("OpenCode: %s", value.Get("error"))
	}
	return event, nil
}

// runCLITurn scopes the command's configuration and session to one Wox request.
func (p *OpenCodeCLIProvider) runCLITurn(ctx context.Context, workspace string, model common.Model, conversations []common.Conversation, effort, bridgeURL string, maxTurns int, emit func(installedEvent)) error {
	args, env, err := p.turnCommand(workspace, model.Name, effort, bridgeURL, maxTurns)
	if err != nil {
		return err
	}
	prompt := installedPrompt(conversations, bridgeURL)
	sessionID := ""
	defer func() {
		if sessionID != "" {
			p.removeSession(ctx, workspace, []string{"session", "delete", sessionID, "--pure"}, env)
		}
	}()
	return p.runJSONTurn(ctx, workspace, args, env, prompt, emit, func(line []byte) (installedEvent, error) {
		if id := gjson.GetBytes(line, "sessionID").String(); id != "" {
			sessionID = id
		}
		return decodeOpenCodeLine(line)
	})
}
