package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"wox/common"
	"wox/setting"
	"wox/util/shell"

	"github.com/tidwall/gjson"
)

func init() { providerFactories["codex-cli"] = NewCodexCLIClient }

type CodexCLIProvider struct{ *CLIBaseProvider }

// NewCodexCLIClient binds the installed Codex app-server protocol.
func NewCodexCLIClient(_ context.Context, config setting.AIProvider) Provider {
	p := &CodexCLIProvider{}
	p.CLIBaseProvider = &CLIBaseProvider{config: config, provider: p, npmEntry: "@openai/codex/bin/codex.js"}
	return p
}

// codexRPC is one private stdio connection; it never changes CODEX_HOME or saved configuration.
type codexRPC struct {
	cmd     *exec.Cmd
	input   io.WriteCloser
	scanner *bufio.Scanner
	stderr  installedErrorBuffer
	nextID  int
	notify  func(gjson.Result) error
}

// startCodex restricts native agent tools while allowing the request's Wox MCP bridge.
func (p *CodexCLIProvider) startCodex(ctx context.Context, workspace, bridgeURL string) (*codexRPC, error) {
	args := []string{"-c", "check_for_update_on_startup=false", "-c", "project_doc_max_bytes=0"}
	for _, feature := range []string{"apps", "plugins", "remote_plugin", "shell_tool", "unified_exec", "browser_use", "in_app_browser", "computer_use", "image_generation", "multi_agent", "hooks"} {
		args = append(args, "-c", "features."+feature+"=false")
	}
	mcpConfig := "mcp_servers={}"
	if bridgeURL != "" {
		// Wox owns tool execution and its enabled-tool boundary. Recent Codex versions
		// require explicit MCP approval even under `never`; approve only this private bridge.
		mcpConfig = fmt.Sprintf("mcp_servers={wox={url=%q,tools={list_tools={approval_mode=\"approve\"},call_tool={approval_mode=\"approve\"}}}}", bridgeURL)
	}
	args = append(args, "-c", mcpConfig, "app-server")
	cmd, err := p.newCommand(ctx, workspace, args, nil)
	if err != nil {
		return nil, err
	}
	rpc := &codexRPC{cmd: cmd}
	cmd.Stderr = &rpc.stderr
	rpc.input, err = cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	rpc.scanner = bufio.NewScanner(output)
	rpc.scanner.Buffer(make([]byte, 65536), 8<<20)
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	if err = shell.AdoptLifetimeBoundCmd(ctx, cmd); err != nil {
		rpc.close()
		return nil, err
	}
	if _, err = rpc.request("initialize", map[string]any{"clientInfo": map[string]string{"name": "wox", "version": "1"}}); err != nil {
		rpc.close()
		return nil, err
	}
	if err = rpc.write(map[string]any{"method": "initialized"}); err != nil {
		rpc.close()
		return nil, err
	}
	account, err := rpc.request("account/read", map[string]any{"refreshToken": false})
	if err != nil {
		rpc.close()
		return nil, err
	}
	if !account.Get("account").IsObject() {
		rpc.close()
		return nil, fmt.Errorf("sign in with codex login")
	}
	return rpc, nil
}

func (r *codexRPC) close()                { _ = r.input.Close(); _ = r.cmd.Cancel(); _ = r.cmd.Wait() }
func (r *codexRPC) write(value any) error { return json.NewEncoder(r.input).Encode(value) }

// read rejects unexpected server-initiated operations; Wox tools are handled by its MCP endpoint.
func (r *codexRPC) read() (gjson.Result, error) {
	if !r.scanner.Scan() {
		if err := r.scanner.Err(); err != nil {
			return gjson.Result{}, err
		}
		return gjson.Result{}, io.ErrUnexpectedEOF
	}
	line := r.scanner.Bytes()
	if !gjson.ValidBytes(line) {
		return gjson.Result{}, fmt.Errorf("Codex returned invalid JSON")
	}
	value := gjson.ParseBytes(line)
	if value.Get("method").Exists() {
		if value.Get("id").Exists() {
			if err := r.write(map[string]any{"id": value.Get("id").Value(), "error": map[string]any{"code": -32601, "message": "Only Wox MCP tools are supported"}}); err != nil {
				return value, err
			}
		} else if r.notify != nil {
			if err := r.notify(value); err != nil {
				return value, err
			}
		}
	}
	return value, nil
}

// request drains interleaved notifications while waiting for the matching response.
func (r *codexRPC) request(method string, params any) (gjson.Result, error) {
	r.nextID++
	id := r.nextID
	if err := r.write(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		return gjson.Result{}, err
	}
	for {
		value, err := r.read()
		if err != nil {
			return value, err
		}
		if value.Get("method").Exists() || !value.Get("id").Exists() || value.Get("id").Int() != int64(id) {
			continue
		}
		if value.Get("error").Exists() {
			return value, fmt.Errorf("Codex %s: %s", method, value.Get("error.message"))
		}
		return value.Get("result"), nil
	}
}

// codexCatalog follows pagination so model selection is not tied to a fixed built-in list.
func codexCatalog(rpc *codexRPC) ([]gjson.Result, error) {
	var models []gjson.Result
	cursor := ""
	for {
		params := map[string]any{"includeHidden": false, "limit": 100}
		if cursor != "" {
			params["cursor"] = cursor
		}
		result, err := rpc.request("model/list", params)
		if err != nil {
			return nil, err
		}
		models = append(models, result.Get("data").Array()...)
		next := result.Get("nextCursor").String()
		if next == "" {
			return models, nil
		}
		if next == cursor || len(models) > 10000 {
			return nil, fmt.Errorf("invalid Codex model pagination")
		}
		cursor = next
	}
}

// modelNames reads the authenticated app-server catalog.
func (p *CodexCLIProvider) modelNames(ctx context.Context, workspace string) ([]string, error) {
	rpc, err := p.startCodex(ctx, workspace, "")
	if err != nil {
		return nil, err
	}
	defer rpc.close()
	catalog, err := codexCatalog(rpc)
	if err != nil {
		return nil, err
	}
	models := make([]string, 0, len(catalog))
	for _, model := range catalog {
		if name := model.Get("model").String(); name != "" {
			models = append(models, name)
		}
	}
	if len(models) == 0 {
		return nil, fmt.Errorf("Codex returned no models")
	}
	return models, nil
}

// runCLITurn scopes reasoning effort to the turn and tears down its ephemeral server after completion.
func (p *CodexCLIProvider) runCLITurn(ctx context.Context, workspace string, model common.Model, conversations []common.Conversation, effort, bridgeURL string, maxTurns int, emit func(installedEvent)) error {
	rpc, err := p.startCodex(ctx, workspace, bridgeURL)
	if err != nil {
		return err
	}
	defer rpc.close()
	if effort != "" {
		catalog, err := codexCatalog(rpc)
		if err != nil {
			return err
		}
		supported := false
		for _, entry := range catalog {
			if entry.Get("model").String() == model.Name {
				for _, level := range entry.Get("supportedReasoningEfforts").Array() {
					supported = supported || level.Get("reasoningEffort").String() == effort
				}
			}
		}
		if !supported {
			return fmt.Errorf("Codex model %s does not support effort %s", model.Name, effort)
		}
	}
	thread, err := rpc.request("thread/start", map[string]any{"model": model.Name, "cwd": workspace, "approvalPolicy": "never", "sandbox": "read-only", "ephemeral": true, "config": map[string]any{"web_search": "disabled"}, "developerInstructions": installedInstructions(bridgeURL)})
	if err != nil {
		return err
	}
	id := thread.Get("thread.id").String()
	if id == "" {
		return fmt.Errorf("Codex returned no thread ID")
	}
	completed := false
	rpc.notify = func(value gjson.Result) error {
		params := value.Get("params")
		if threadID := params.Get("threadId").String(); threadID != "" && threadID != id {
			return nil
		}
		switch value.Get("method").String() {
		case "item/agentMessage/delta":
			emit(installedEvent{text: params.Get("delta").String()})
		case "item/reasoning/summaryTextDelta":
			emit(installedEvent{reasoning: params.Get("delta").String()})
		case "turn/completed":
			if params.Get("turn.status").String() != "completed" {
				return fmt.Errorf("Codex turn %s: %s", params.Get("turn.status"), params.Get("turn.error.message"))
			}
			completed = true
		case "error":
			if !params.Get("willRetry").Bool() {
				return fmt.Errorf("Codex: %s", params.Get("error.message"))
			}
		}
		return nil
	}
	params := map[string]any{"threadId": id, "model": model.Name, "approvalPolicy": "never", "input": []any{map[string]any{"type": "text", "text": installedPrompt(conversations, bridgeURL)}}}
	if effort != "" {
		params["effort"] = effort
	}
	if _, err = rpc.request("turn/start", params); err != nil {
		return err
	}
	for !completed {
		if _, err = rpc.read(); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("Codex response: %w", err)
		}
	}
	return nil
}
