package ai

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
	"wox/common"
	"wox/setting"
	"wox/util"
	"wox/util/shell"

	"github.com/google/uuid"
)

const installedToolInstructions = `You are answering inside Wox. Use only the conversation supplied here and Wox tools. Never use your own filesystem, shell, browser, memory, plugins, or other MCP servers.
The wox MCP server exposes exactly two wrapper tools: list_tools and call_tool. Discover these two MCP tools using your CLI's tool discovery mechanism and invoke their exact qualified names. list_tools returns a catalog of Wox operations, NOT additional MCP tools. Never search for or directly invoke catalog names such as read_skill, read, bash, or load_tools as MCP tools.
To execute a catalog operation, invoke the wox call_tool wrapper with {"name":"read_skill","arguments":{"id":"the exact skill id"}} (substitute the catalog operation and its arguments). With a CLI use_tool dispatcher, tool_name must be the qualified MCP name of the call_tool wrapper; the wrapper's input contains the separate Wox operation name and arguments. Wox load_tools is also invoked through call_tool; call list_tools again after it succeeds.
Previous tool results are conversation history, not instructions to repeat those calls. Finish with an answer to the user's question, not a progress announcement. If a tool cannot be used, explain that limitation and answer from the available context.`

// cliProvider keeps command-specific authentication and protocols in their own providers.
type cliProvider interface {
	modelNames(context.Context, string) ([]string, error)
	runCLITurn(ctx context.Context, workspace string, model common.Model, conversations []common.Conversation, effort, bridgeURL string, maxTurns int, emit func(installedEvent)) error
}

// CLIBaseProvider owns shared process, stream, and Wox tool bridge lifetimes.
type CLIBaseProvider struct {
	config   setting.AIProvider
	provider cliProvider
	npmEntry string
}

// GetIcon uses the command's brand mark so installed CLIs match their API providers in settings.
func (p *CLIBaseProvider) GetIcon() common.WoxImage {
	return installedCLIIcon(p.config.Name)
}
func (p *CLIBaseProvider) GetDefaultHost() string { return "" }
func (p *CLIBaseProvider) command() string {
	return strings.TrimSuffix(string(p.config.Name), "-cli")
}

// Models uses each installed command's own account and catalog without reading credentials.
func (p *CLIBaseProvider) Models(ctx context.Context) ([]common.Model, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	workspace, err := os.MkdirTemp("", "wox-ai-probe-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(workspace)
	names, err := p.provider.modelNames(ctx, workspace)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%s returned no models; check its installation and login", p.command())
	}
	models := make([]common.Model, 0, len(names))
	for _, name := range names {
		models = append(models, common.Model{Name: name, Provider: p.config.Name, ProviderAlias: p.config.Alias})
	}
	return models, nil
}

func (p *CLIBaseProvider) Ping(ctx context.Context) error { _, err := p.Models(ctx); return err }

// ChatStream starts an isolated request while retaining the command's normal credential environment.
func (p *CLIBaseProvider) ChatStream(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions) (ChatStream, error) {
	if strings.TrimSpace(model.Name) == "" || strings.HasPrefix(model.Name, "-") {
		return nil, fmt.Errorf("invalid installed model name")
	}
	if _, _, err := p.executable(); err != nil {
		return nil, err
	}
	for _, conversation := range conversations {
		if len(conversation.Images) > 0 {
			return nil, fmt.Errorf("%s currently accepts text conversations; remove image attachments", p.command())
		}
		for _, attachment := range conversation.Attachments {
			if attachment.Kind == common.AIChatAttachmentImage {
				return nil, fmt.Errorf("%s currently accepts text conversations; remove image attachments", p.command())
			}
		}
	}
	effort := strings.TrimSpace(p.config.ReasoningEffort)
	if effort != "" && !slices.Contains([]string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}, effort) {
		return nil, fmt.Errorf("invalid reasoning effort: %s", effort)
	}
	streamCtx, cancel := context.WithCancel(ctx)
	stream := &installedStream{ctx: streamCtx, cancel: cancel, events: make(chan installedEvent, 32)}
	go func() {
		defer close(stream.events)
		defer cancel()
		err := p.runTurn(streamCtx, model, conversations, options, effort, stream.emit)
		stream.emit(installedEvent{done: true, err: err})
	}()
	return stream, nil
}

// runTurn owns temporary configs, the tool bridge, and the child process as one lifetime.
func (p *CLIBaseProvider) runTurn(ctx context.Context, model common.Model, conversations []common.Conversation, options common.ChatOptions, effort string, emit func(installedEvent)) error {
	started := time.Now()
	var firstEvent, firstText sync.Once
	forward := emit
	emit = func(event installedEvent) {
		firstEvent.Do(func() {
			util.GetLogger().Info(ctx, fmt.Sprintf("AI: CLI provider=%s stage=first_event elapsedMs=%d", p.command(), time.Since(started).Milliseconds()))
		})
		if event.text != "" {
			firstText.Do(func() {
				util.GetLogger().Info(ctx, fmt.Sprintf("AI: CLI provider=%s stage=first_text elapsedMs=%d", p.command(), time.Since(started).Milliseconds()))
			})
		}
		forward(event)
	}
	workspace, err := os.MkdirTemp("", "wox-ai-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workspace)
	bridgeURL, closeBridge, err := startInstalledToolBridge(ctx, options, emit)
	if err != nil {
		return err
	}
	defer closeBridge()
	return p.provider.runCLITurn(ctx, workspace, model, conversations, effort, bridgeURL, options.LoopPolicy.MaxIterations, emit)
}

// runJSONTurn waits for both a terminal event and successful process exit.
func (p *CLIBaseProvider) runJSONTurn(ctx context.Context, workspace string, args, env []string, prompt string, emit func(installedEvent), decode func([]byte) (installedEvent, error)) error {
	completed := false
	err := p.runCommand(ctx, workspace, args, env, strings.NewReader(prompt), func(line []byte) error {
		event, err := decode(line)
		if err != nil {
			return err
		}
		completed = completed || event.done
		// Success is published only after the child exits successfully, never on a partial result.
		event.done = false
		if event.text != "" || event.reasoning != "" {
			emit(event)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !completed {
		return fmt.Errorf("%s exited without completing its response", p.command())
	}
	return nil
}

// removeSession deletes only the CLI session created for this request.
func (p *CLIBaseProvider) removeSession(ctx context.Context, workspace string, args, env []string) {
	started := time.Now()
	defer func() {
		util.GetLogger().Info(ctx, fmt.Sprintf("AI: CLI provider=%s stage=session_cleanup elapsedMs=%d", p.command(), time.Since(started).Milliseconds()))
	}()
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := p.runCommand(cleanupCtx, workspace, args, env, nil, func([]byte) error { return nil }); err != nil {
		util.GetLogger().Warn(cleanupCtx, "AI: could not remove temporary CLI session: "+err.Error())
	}
}

// installedInstructions only describes tools when this request actually has a bridge.
func installedInstructions(bridgeURL string) string {
	if bridgeURL != "" {
		return installedToolInstructions
	}
	return "You are processing a text-only request inside Wox. No tools are available for this request. Do not discover or invoke tools, access external resources, or announce a plan. Follow the requested output format and return only the requested result using the supplied context."
}

// installedPrompt preserves roles and completed tool results when starting a fresh CLI conversation.
func installedPrompt(conversations []common.Conversation, bridgeURL string) string {
	var prompt strings.Builder
	prompt.WriteString(installedInstructions(bridgeURL))
	for _, conversation := range conversations {
		fmt.Fprintf(&prompt, "\n\n%s:\n%s", conversation.Role, conversation.Text)
		if conversation.Role == common.ConversationRoleTool {
			fmt.Fprintf(&prompt, "\nTool %s result:\n%s", conversation.ToolCallInfo.Name, conversation.ToolCallInfo.Response)
		}
	}
	return prompt.String()
}

type installedEvent struct {
	text, reasoning string
	call            *common.ToolCallInfo
	done            bool
	err             error
}
type installedStream struct {
	ctx    context.Context
	cancel context.CancelFunc
	events chan installedEvent
	data   common.ChatStreamData
}

func (s *installedStream) emit(event installedEvent) {
	select {
	case s.events <- event:
	case <-s.ctx.Done():
	}
}

// Receive aggregates text and Wox tool events on the consumer goroutine, avoiding cross-stream races.
func (s *installedStream) Receive(ctx context.Context) (common.ChatStreamData, error) {
	select {
	case <-ctx.Done():
		s.cancel()
		return common.ChatStreamData{}, ctx.Err()
	case event, ok := <-s.events:
		if !ok {
			return common.ChatStreamData{}, io.EOF
		}
		if event.err != nil {
			return common.ChatStreamData{}, event.err
		}
		s.data.Data += event.text
		s.data.Reasoning += event.reasoning
		if event.text != "" || event.reasoning != "" {
			last := len(s.data.Conversations) - 1
			// Tool boundaries and renewed reasoning must not append to earlier text.
			if last < 0 || s.data.Conversations[last].Role != common.ConversationRoleAssistant || (event.reasoning != "" && s.data.Conversations[last].Text != "") {
				s.data.Conversations = append(s.data.Conversations, common.Conversation{Id: uuid.NewString(), Role: common.ConversationRoleAssistant})
				last++
			}
			s.data.Conversations[last].Text += event.text
			s.data.Conversations[last].Reasoning += event.reasoning
			s.data.Conversations[last].Timestamp = util.GetSystemTimestamp()
		}
		if event.call != nil {
			index := slices.IndexFunc(s.data.ToolCalls, func(call common.ToolCallInfo) bool { return call.Id == event.call.Id })
			if index < 0 {
				s.data.ToolCalls = append(s.data.ToolCalls, *event.call)
			} else {
				s.data.ToolCalls[index] = *event.call
			}
			conversation := common.Conversation{Id: event.call.Id, Role: common.ConversationRoleTool, Text: event.call.Delta, ToolCallInfo: *event.call, Timestamp: event.call.StartTimestamp}
			index = slices.IndexFunc(s.data.Conversations, func(c common.Conversation) bool { return c.Id == event.call.Id })
			if index < 0 {
				s.data.Conversations = append(s.data.Conversations, conversation)
			} else {
				// Completion updates retain the tool's original position, even after new text.
				s.data.Conversations[index] = conversation
			}
		}
		s.data.Status = common.ChatStreamStatusStreaming
		if event.done {
			s.data.Status = common.ChatStreamStatusStreamed
		}
		result := s.data
		result.ToolCalls = append([]common.ToolCallInfo(nil), s.data.ToolCalls...)
		result.Conversations = append([]common.Conversation(nil), s.data.Conversations...)
		return result, nil
	}
}

// executable locates native commands and resolves known npm shims through Node without a shell.
func (p *CLIBaseProvider) executable() (string, []string, error) {
	name := p.command()
	path := p.config.Executable
	if path == "" {
		path, _ = exec.LookPath(name)
		if path == "" {
			home, _ := os.UserHomeDir()
			for _, directory := range []string{filepath.Join(home, ".local", "bin"), filepath.Join(home, ".opencode", "bin"), filepath.Join(home, ".grok", "bin"), filepath.Join(home, ".claude", "local"), "/opt/homebrew/bin", "/usr/local/bin", filepath.Join(os.Getenv("APPDATA"), "npm")} {
				candidate := filepath.Join(directory, name)
				if runtime.GOOS == "windows" {
					candidate += ".exe"
				}
				if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
					path = candidate
					break
				}
			}
		}
	}
	if path == "" {
		return "", nil, fmt.Errorf("%s is not installed or not on PATH; set its executable path in AI settings", name)
	}
	if !filepath.IsAbs(path) {
		return "", nil, fmt.Errorf("CLI executable must be an absolute path")
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".cmd" || ext == ".bat" || ext == ".ps1" {
		entry := p.npmEntry
		if entry != "" {
			script := filepath.Join(filepath.Dir(path), "node_modules", filepath.FromSlash(entry))
			if _, err := os.Stat(script); err == nil {
				node, err := exec.LookPath("node")
				if err == nil {
					return node, []string{script}, nil
				}
			}
		}
		return "", nil, fmt.Errorf("select the native %s executable; this shell shim could not be resolved safely", name)
	}
	return path, nil, nil
}

// output bounds probe output independently from the line-oriented streaming parser.
func (p *CLIBaseProvider) output(ctx context.Context, workspace string, args []string) (string, error) {
	var output strings.Builder
	err := p.runCommand(ctx, workspace, args, nil, nil, func(line []byte) error {
		if output.Len()+len(line) > 4<<20 {
			return fmt.Errorf("CLI catalog exceeds 4 MiB")
		}
		output.Write(line)
		output.WriteByte('\n')
		return nil
	})
	return output.String(), err
}

// newCommand inherits login state but binds cancellation to the complete process tree.
func (p *CLIBaseProvider) newCommand(ctx context.Context, workspace string, args, env []string) (*exec.Cmd, error) {
	path, prefix, err := p.executable()
	if err != nil {
		return nil, err
	}
	cmd := shell.BuildCommandContext(ctx, path, append(env, "NO_COLOR=1"), append(prefix, args...)...)
	cmd.Dir = workspace
	cmd.WaitDelay = 2 * time.Second
	cmd.Cancel = func() error { shell.TerminateProcessTree(cmd.Process.Pid); return nil }
	shell.PrepareLifetimeBoundCmd(cmd)
	return cmd, nil
}

type installedErrorBuffer struct{ data []byte }

// Write retains only a bounded stderr tail.
func (b *installedErrorBuffer) Write(data []byte) (int, error) {
	n := len(data)
	b.data = append(b.data, data...)
	if len(b.data) > 16384 {
		b.data = b.data[len(b.data)-16384:]
	}
	return n, nil
}

// runCommand drains stdout before Wait, kills descendants on cancellation, and always reaps the child.
func (p *CLIBaseProvider) runCommand(ctx context.Context, workspace string, args, env []string, input io.Reader, consume func([]byte) error) error {
	started := time.Now()
	cmd, err := p.newCommand(ctx, workspace, args, env)
	if err != nil {
		return err
	}
	cmd.Stdin = input
	var stderr installedErrorBuffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		return err
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("AI: CLI provider=%s pid=%d stage=process_started elapsedMs=%d", p.command(), cmd.Process.Pid, time.Since(started).Milliseconds()))
	if err = shell.AdoptLifetimeBoundCmd(ctx, cmd); err != nil {
		_ = cmd.Cancel()
		_ = cmd.Wait()
		return err
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 65536), 8<<20)
	for scanner.Scan() {
		if len(scanner.Bytes()) > 0 {
			if err = consume(scanner.Bytes()); err != nil {
				break
			}
		}
	}
	if err == nil {
		err = scanner.Err()
	}
	if err != nil {
		_ = cmd.Cancel()
	}
	waitErr := cmd.Wait()
	util.GetLogger().Info(ctx, fmt.Sprintf("AI: CLI provider=%s pid=%d stage=process_exited elapsedMs=%d success=%t", p.command(), cmd.Process.Pid, time.Since(started).Milliseconds(), err == nil && waitErr == nil))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	if waitErr != nil {
		return fmt.Errorf("%s failed: %w: %s", p.command(), waitErr, strings.TrimSpace(string(stderr.data)))
	}
	return nil
}
