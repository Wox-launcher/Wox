package ai

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"wox/common"
	"wox/setting"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tidwall/gjson"
	"github.com/tmc/langchaingo/jsonschema"
)

// TestInstalledStreamConversationOrder covers multiple tool rounds within one CLI turn.
func TestInstalledStreamConversationOrder(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &installedStream{ctx: ctx, cancel: cancel, events: make(chan installedEvent, 1)}
	events := []installedEvent{
		{text: "before"},
		{call: &common.ToolCallInfo{Id: "one", Status: common.ToolCallStatusRunning}},
		{text: "after"},
		{call: &common.ToolCallInfo{Id: "one", Status: common.ToolCallStatusSucceeded}},
		{text: " tool"},
		{reasoning: "next thought"},
		{call: &common.ToolCallInfo{Id: "two", Status: common.ToolCallStatusRunning}},
		{call: &common.ToolCallInfo{Id: "two", Status: common.ToolCallStatusSucceeded}},
		{reasoning: "final thought"},
		{text: "answer"},
		{done: true},
	}
	var snapshots []common.ChatStreamData
	for _, event := range events {
		stream.emit(event)
		result, err := stream.Receive(ctx)
		if err != nil {
			t.Fatal(err)
		}
		snapshots = append(snapshots, result)
	}
	result := snapshots[len(snapshots)-1]
	if result.Data != "beforeafter toolanswer" || result.Status != common.ChatStreamStatusStreamed {
		t.Fatalf("legacy aggregate changed: %+v", result)
	}
	conversations := result.Conversations
	if len(conversations) != 6 || conversations[0].Text != "before" || conversations[1].Id != "one" || conversations[2].Text != "after tool" || conversations[3].Reasoning != "next thought" || conversations[4].Id != "two" || conversations[5].Text != "answer" || conversations[5].Reasoning != "final thought" {
		t.Fatalf("conversation order lost: %+v", conversations)
	}
	if conversations[1].ToolCallInfo.Status != common.ToolCallStatusSucceeded || conversations[4].ToolCallInfo.Status != common.ToolCallStatusSucceeded {
		t.Fatalf("tool completion lost: %+v", conversations)
	}
	if snapshots[1].Conversations[1].ToolCallInfo.Status != common.ToolCallStatusRunning || snapshots[2].Conversations[2].Text != "after" || snapshots[2].Conversations[2].Id != conversations[2].Id {
		t.Fatal("stream snapshots mutated or message identity changed")
	}
}

// TestInstalledTextOnlyPrompt keeps title/summary requests out of tool discovery.
func TestInstalledTextOnlyPrompt(t *testing.T) {
	conversations := []common.Conversation{{Role: common.ConversationRoleUser, Text: "今天伦敦天气如何？"}}
	text := installedPrompt(conversations, "")
	if strings.Contains(text, "list_tools") || strings.Contains(text, "call_tool") || !strings.Contains(text, conversations[0].Text) {
		t.Fatalf("unexpected text-only prompt: %s", text)
	}
	if !strings.Contains(installedPrompt(conversations, "http://bridge"), "call_tool") {
		t.Fatal("tool-enabled prompt lost routing instructions")
	}
	p := &GrokCLIProvider{}
	args, _, err := p.turnCommand(t.TempDir(), "grok-4.6", "", "", 25)
	if err != nil {
		t.Fatal(err)
	}
	for i, arg := range args {
		if arg == "--tools" && args[i+1] != "" {
			t.Fatal("text-only Grok request enables tool discovery")
		}
	}
}

// TestGrokStopReason rejects interrupted turns even when the CLI exits successfully.
func TestGrokStopReason(t *testing.T) {
	for _, reason := range []string{"end_turn", "max_tokens", "max_turn_requests", "cancelled", ""} {
		event, err := decodeGrokLine([]byte(fmt.Sprintf(`{"type":"end","stopReason":%q}`, reason)))
		if reason == "end_turn" {
			if err != nil || !event.done {
				t.Fatalf("completed turn rejected: %v", err)
			}
		} else if err == nil || event.done {
			t.Fatalf("interrupted turn accepted: %q", reason)
		}
	}
}

// TestGrokLiveToolRouting is opt-in because it uses the locally authenticated model.
func TestGrokLiveToolRouting(t *testing.T) {
	if os.Getenv("WOX_TEST_GROK_LIVE") != "1" {
		t.Skip("set WOX_TEST_GROK_LIVE=1 to exercise the installed Grok")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	provider := NewGrokCLIClient(ctx, setting.AIProvider{Name: "grok-cli"})
	marker := "WOX-" + fmt.Sprint(time.Now().UnixNano())
	var calls atomic.Int32
	options := common.ChatOptions{Tools: []common.Tool{{Name: "read_skill", Description: "Read the Wox plugin skill definition.", Source: common.ToolSourceBuiltin,
		Parameters: jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{"id": {Type: jsonschema.String}}, Required: []string{"id"}}}},
		ExecuteTool: func(_ context.Context, option common.AgentToolExecutionOption) common.AgentToolExecutionResult {
			calls.Add(1)
			option.Call.Status = common.ToolCallStatusSucceeded
			option.Call.Response = "A single-file plugin contains its metadata and implementation in one source file. Verification code: " + marker
			option.OnUpdate(option.Call)
			return common.AgentToolExecutionResult{Call: option.Call, Result: common.ToolResult{Text: option.Call.Response}}
		}}
	stream, err := provider.ChatStream(ctx, common.Model{Name: "grok-4.6", Provider: "grok-cli"}, []common.Conversation{{Role: common.ConversationRoleUser, Text: "Read the Wox plugin skill with id builtin:wox-plugin-creator, then explain what a single-file plugin means. Include the verification code from the skill in your final answer."}}, options)
	if err != nil {
		t.Fatal(err)
	}
	for {
		result, err := stream.Receive(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status == common.ChatStreamStatusStreamed {
			if calls.Load() == 0 || !strings.Contains(result.Data, marker) {
				t.Fatalf("tool routing or answer incomplete: calls=%d text=%q", calls.Load(), result.Data)
			}
			return
		}
	}
}

// TestMain doubles as an installed CLI process so cancellation and pipe framing are exercised for real.
func TestMain(m *testing.M) {
	if mode := os.Getenv("WOX_TEST_CLI_HELPER"); mode != "" {
		if err := installedCLIHelper(mode); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// installedCLIHelper emulates the published CLI wire formats, including a real MCP client call.
func installedCLIHelper(mode string) error {
	args := os.Args[1:]
	argument := func(name string) string {
		for i, arg := range args {
			if arg == name && i+1 < len(args) {
				return args[i+1]
			}
		}
		return ""
	}
	if mode == "hang" {
		time.Sleep(time.Minute)
		return nil
	}
	if strings.Contains(strings.Join(args, " "), "session delete") || strings.Contains(strings.Join(args, " "), "sessions delete") {
		return nil
	}
	if mode == "codex-cli" {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			value := gjson.ParseBytes(scanner.Bytes())
			result := any(map[string]any{})
			switch value.Get("method").String() {
			case "initialized":
				continue
			case "account/read":
				result = map[string]any{"account": map[string]string{"type": "chatgpt"}}
			case "model/list":
				result = map[string]any{"data": []any{map[string]any{"model": "test-model", "supportedReasoningEfforts": []any{map[string]string{"reasoningEffort": "high"}}}}}
			case "thread/start":
				result = map[string]any{"thread": map[string]string{"id": "thread-1"}}
			case "turn/start":
				for i, arg := range args {
					if arg == "-c" && i+1 < len(args) && strings.HasPrefix(args[i+1], "mcp_servers={wox=") {
						config := args[i+1]
						start := strings.Index(config, "http://")
						end := strings.Index(config[start:], `"`)
						if err := installedHelperToolCall(config[start : start+end]); err != nil {
							return err
						}
					}
				}
				fmt.Println(`{"method":"item/agentMessage/delta","params":{"threadId":"thread-1","delta":"hello"}}`)
				fmt.Println(`{"method":"turn/completed","params":{"threadId":"thread-1","turn":{"status":"completed"}}}`)
			}
			if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"id": value.Get("id").Int(), "result": result}); err != nil {
				return err
			}
		}
		return scanner.Err()
	}
	switch mode {
	case "claude-cli":
		if argument("auth") == "status" {
			fmt.Println(`{"loggedIn":true}`)
			return nil
		}
		url := gjson.Get(argument("--mcp-config"), "mcpServers.wox.url").String()
		if url != "" {
			if err := installedHelperToolCall(url); err != nil {
				return err
			}
		}
		if _, err := io.ReadAll(os.Stdin); err != nil {
			return err
		}
		fmt.Println(`{"type":"stream_event","event":{"delta":{"type":"text_delta","text":"hello"}}}`)
		fmt.Println(`{"type":"result","is_error":false,"result":"hello"}`)
	case "opencode-cli":
		if len(args) > 0 && args[0] == "models" {
			fmt.Println("test/model\n{\"variants\":{\"high\":{}}}")
			return nil
		}
		url := gjson.Get(os.Getenv("OPENCODE_CONFIG_CONTENT"), "mcp.wox.url").String()
		if url != "" {
			if err := installedHelperToolCall(url); err != nil {
				return err
			}
		}
		fmt.Println(`{"type":"text","sessionID":"ses_test","part":{"text":"hello"}}`)
		fmt.Println(`{"type":"step_finish","part":{"reason":"stop"}}`)
	case "grok-cli":
		if len(args) > 0 && args[len(args)-1] == "models" {
			fmt.Println("You are logged in with grok.com.\nAvailable models:\n  * test-model (default)")
			return nil
		}
		if _, err := os.ReadFile(argument("--prompt-file")); err != nil {
			return err
		}
		config, err := os.ReadFile(filepath.Join(argument("--cwd"), ".grok", "config.toml"))
		if err == nil {
			text := string(config)
			start := strings.Index(text, "http://")
			end := strings.Index(text[start:], `"`)
			if err := installedHelperToolCall(text[start : start+end]); err != nil {
				return err
			}
		}
		fmt.Println(`{"type":"thought","data":"thinking"}`)
		fmt.Println(`{"type":"text","data":"hello"}`)
		fmt.Println(`{"type":"end","stopReason":"end_turn"}`)
	}
	return nil
}

// installedHelperToolCall verifies the agent can discover and invoke a Wox tool through HTTP MCP.
func installedHelperToolCall(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mcp.NewClient(&mcp.Implementation{Name: "cli-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		return err
	}
	defer session.Close()
	if _, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "list_tools", Arguments: map[string]any{}}); err != nil {
		return err
	}
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "call_tool", Arguments: map[string]any{"name": "echo", "arguments": map[string]any{"text": "from CLI"}}})
	if err != nil {
		return err
	}
	if result.IsError {
		return fmt.Errorf("tool call failed: %+v", result)
	}
	if len(result.Content) != 1 || result.Content[0].(*mcp.TextContent).Text != "from CLI" {
		return fmt.Errorf("unexpected tool response: %+v", result)
	}
	return nil
}

// TestInstalledProvidersModelDiscoveryAndToolStreaming exercises all four process transports.
func TestInstalledProvidersModelDiscoveryAndToolStreaming(t *testing.T) {
	for _, name := range []common.ProviderName{"claude-cli", "codex-cli", "opencode-cli", "grok-cli"} {
		t.Run(string(name), func(t *testing.T) {
			t.Setenv("WOX_TEST_CLI_HELPER", string(name))
			path, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			provider := providerFactories[name](context.Background(), setting.AIProvider{Name: name, Alias: "personal", Executable: path})
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			models, err := provider.Models(ctx)
			if err != nil || len(models) == 0 {
				t.Fatalf("models: %v %v", models, err)
			}
			if models[0].ProviderAlias != "personal" || models[0].Provider != name {
				t.Fatal(models[0])
			}
			var calls atomic.Int32
			tool := common.Tool{Name: "echo", Source: common.ToolSourceBuiltin, Parameters: jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{"text": {Type: jsonschema.String}}, Required: []string{"text"}}, Callback: func(_ context.Context, args map[string]any) (common.ToolResult, error) {
				calls.Add(1)
				return common.ToolResult{Text: args["text"].(string)}, nil
			}}
			options := common.ChatOptions{Tools: []common.Tool{tool}, ExecuteTool: func(ctx context.Context, option common.AgentToolExecutionOption) common.AgentToolExecutionResult {
				option.Call.Status = common.ToolCallStatusRunning
				option.OnUpdate(option.Call)
				result, err := option.Tool.Callback(ctx, option.Call.Arguments)
				if err != nil {
					t.Error(err)
				}
				option.Call.Status = common.ToolCallStatusSucceeded
				option.Call.Response = result.Text
				option.OnUpdate(option.Call)
				return common.AgentToolExecutionResult{Result: result, Call: option.Call}
			}}
			stream, err := provider.ChatStream(ctx, models[0], []common.Conversation{{Role: common.ConversationRoleUser, Text: "hello"}}, options)
			if err != nil {
				t.Fatal(err)
			}
			for {
				result, err := stream.Receive(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if result.Status == common.ChatStreamStatusStreamed {
					if result.Data != "hello" || calls.Load() != 1 || len(result.ToolCalls) != 1 || result.ToolCalls[0].Status != common.ToolCallStatusSucceeded {
						t.Fatalf("result=%+v calls=%d", result, calls.Load())
					}
					break
				}
			}
		})
	}
}

// TestInstalledCommandCancellation ensures a silent child cannot strand a cancelled chat.
func TestInstalledCommandCancellation(t *testing.T) {
	t.Setenv("WOX_TEST_CLI_HELPER", "hang")
	path, _ := os.Executable()
	p := &CLIBaseProvider{config: setting.AIProvider{Name: "claude-cli", Executable: path}}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	started := time.Now()
	err := p.runCommand(ctx, t.TempDir(), nil, nil, nil, func([]byte) error { return nil })
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 5*time.Second {
		t.Fatalf("cancel: %v after %v", err, time.Since(started))
	}
}

// TestInstalledBridgeRejectsUntrustedRequests checks token and browser-origin boundaries.
func TestInstalledBridgeRejectsUntrustedRequests(t *testing.T) {
	options := common.ChatOptions{Tools: []common.Tool{{Name: "test"}}, ExecuteTool: func(context.Context, common.AgentToolExecutionOption) common.AgentToolExecutionResult {
		t.Fatal("must not execute")
		return common.AgentToolExecutionResult{}
	}}
	url, closeBridge, err := startInstalledToolBridge(context.Background(), options, func(installedEvent) {})
	if err != nil {
		t.Fatal(err)
	}
	defer closeBridge()
	for _, origin := range []string{"", "https://untrusted.example"} {
		endpoint := url
		if origin == "" {
			endpoint += "invalid"
		}
		req, _ := http.NewRequest("POST", endpoint, strings.NewReader(`{}`))
		req.Header.Set("Origin", origin)
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatal(response.Status)
		}
	}
}

// TestInstalledStreamErrorsAndEffort checks terminal errors and provider-specific effort flags.
func TestInstalledStreamErrorsAndEffort(t *testing.T) {
	for _, test := range []struct {
		decode func([]byte) (installedEvent, error)
		line   string
	}{
		{decodeClaudeCodeLine, `{"type":"result","is_error":true,"result":"login required"}`},
		{decodeOpenCodeLine, `{"type":"error","error":{"message":"login required"}}`},
		{decodeGrokLine, `{"type":"error","message":"login required"}`},
		{decodeGrokLine, `not json`},
	} {
		if _, err := test.decode([]byte(test.line)); err == nil {
			t.Fatalf("accepted error: %s", test.line)
		}
	}
	for _, name := range []common.ProviderName{"claude-cli", "opencode-cli", "grok-cli"} {
		p := providerFactories[name](context.Background(), setting.AIProvider{Name: name})
		args, _, err := p.(interface {
			turnCommand(string, string, string, string, int) ([]string, []string, error)
		}).turnCommand(t.TempDir(), "test", "high", "", 25)
		if err != nil {
			t.Fatal(err)
		}
		flag := "--effort"
		if name == "opencode-cli" {
			flag = "--variant"
		}
		if !strings.Contains(strings.Join(args, " "), flag+" high") {
			t.Fatal(args)
		}
	}
}

// TestInstalledBridgeDynamicTools preserves load_tools and rejects unavailable or invalid calls.
func TestInstalledBridgeDynamicTools(t *testing.T) {
	const name = "installed_test_remote_echo"
	remote := common.Tool{Name: name, Source: common.ToolSourceMCP, ServerConfig: &common.AIChatMCPServerConfig{Name: "test-server"}, Parameters: jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{"text": {Type: jsonschema.String}}, Required: []string{"text"}}}
	GetToolRegistry().Register(remote)
	defer GetToolRegistry().Unregister(name)
	loader := common.Tool{Name: LoadToolsToolName, Source: common.ToolSourceBuiltin, Parameters: jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{"name": {Type: jsonschema.String}}}}
	calls := 0
	b := &installedToolBridge{ctx: context.Background(), tools: []common.Tool{loader}, emit: func(installedEvent) {}, options: common.ChatOptions{ExecuteTool: func(_ context.Context, option common.AgentToolExecutionOption) common.AgentToolExecutionResult {
		calls++
		option.Call.Status = common.ToolCallStatusSucceeded
		if option.Tool.Name == name && (option.Call.Source != common.ToolSourceMCP || option.Call.Server != "test-server") {
			t.Fatal("MCP origin was lost")
		}
		return common.AgentToolExecutionResult{Call: option.Call}
	}}}
	invoke := func(name string, args map[string]any) error {
		_, _, err := b.call(context.Background(), nil, installedToolCall{Name: name, Arguments: args})
		return err
	}
	if err := invoke(name, map[string]any{"text": "test"}); err == nil {
		t.Fatal("unloaded tool was callable")
	}
	if err := invoke(LoadToolsToolName, map[string]any{"name": name}); err != nil {
		t.Fatal(err)
	}
	if err := invoke(name, map[string]any{"text": 42}); err == nil {
		t.Fatal("invalid tool arguments were accepted")
	}
	if err := invoke(name, map[string]any{"text": "test"}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("executed %d calls, want loader and remote tool only", calls)
	}
	b.options.LoopPolicy.MaxIterations = 2
	if err := invoke(name, map[string]any{"text": "test"}); err == nil {
		t.Fatal("tool budget ignored")
	}
}

// TestInstalledRealCatalog is opt-in and performs no model generation or credential-file reads.
func TestInstalledRealCatalog(t *testing.T) {
	name := os.Getenv("WOX_TEST_INSTALLED_PROVIDER")
	if name == "" {
		t.Skip("set WOX_TEST_INSTALLED_PROVIDER to probe an installed CLI")
	}
	p := providerFactories[common.ProviderName(name)](context.Background(), setting.AIProvider{Name: common.ProviderName(name)})
	models, err := p.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: %d available models", name, len(models))
}

// TestInstalledRealToolRoundTrip verifies real CLI flags and MCP transport with a side-effect-free tool.
func TestInstalledRealToolRoundTrip(t *testing.T) {
	name := os.Getenv("WOX_TEST_INSTALLED_TOOL_PROVIDER")
	if name == "" {
		t.Skip("set WOX_TEST_INSTALLED_TOOL_PROVIDER to run one real model/tool request")
	}
	p := providerFactories[common.ProviderName(name)](context.Background(), setting.AIProvider{Name: common.ProviderName(name)})
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	models, err := p.Models(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	tool := common.Tool{Name: "echo", Source: common.ToolSourceBuiltin, Description: "Return the text unchanged; has no side effects", Parameters: jsonschema.Definition{Type: jsonschema.Object, Properties: map[string]jsonschema.Definition{"text": {Type: jsonschema.String}}, Required: []string{"text"}}, Callback: func(_ context.Context, args map[string]any) (common.ToolResult, error) {
		calls.Add(1)
		return common.ToolResult{Text: args["text"].(string)}, nil
	}}
	options := common.ChatOptions{Tools: []common.Tool{tool}, ExecuteTool: func(ctx context.Context, option common.AgentToolExecutionOption) common.AgentToolExecutionResult {
		result, err := option.Tool.Callback(ctx, option.Call.Arguments)
		if err != nil {
			t.Error(err)
		}
		option.Call.Status = common.ToolCallStatusSucceeded
		option.Call.Response = result.Text
		option.OnUpdate(option.Call)
		return common.AgentToolExecutionResult{Result: result, Call: option.Call}
	}}
	stream, err := p.ChatStream(ctx, models[0], []common.Conversation{{Role: common.ConversationRoleUser, Text: "Integration test: call the wox MCP list_tools tool, then its call_tool tool with name echo and arguments {\"text\":\"WOX_OK\"}. Do not use any other tools. Reply with only the tool result."}}, options)
	if err != nil {
		t.Fatal(err)
	}
	for {
		result, err := stream.Receive(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if result.Status == common.ChatStreamStatusStreamed {
			if calls.Load() != 1 || !strings.Contains(result.Data, "WOX_OK") {
				t.Fatalf("tool calls=%d response=%q", calls.Load(), result.Data)
			}
			t.Logf("%s: model %s completed a Wox tool call", name, models[0].Name)
			return
		}
	}
}
