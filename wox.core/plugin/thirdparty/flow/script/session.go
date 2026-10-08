package script

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"wox/util"
	"wox/util/shell"

	_ "embed"
)

//go:embed client/flowlauncher/__init__.py
var flowLauncherClientSource string

var errFlowProcessExited = errors.New("plugin process exited")

const flowCallTimeout = 60 * time.Second

type flowInbound struct {
	id      string
	reply   flowReply
	err     error
	exited  bool
	pending bool
}

// flowSession talks to one plugin process.
type flowSession struct {
	name       string
	directory  string
	entry      string
	language   string
	dialect    string
	pythonPath string
	nodePath   string
	clientDir  string
	bridge     flowBridge

	mu      sync.Mutex
	writeMu sync.Mutex
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	alive   bool
	closed  bool
	nextID  int64
	pending map[string]chan flowInbound
	done    chan struct{}
	exitErr error
	stderr  bytes.Buffer
	everV2  bool
	callCtx context.Context
}

func newFlowSession(name, directory, entry, language, dialect, pythonPath, nodePath, clientDir string, bridge flowBridge) *flowSession {
	return &flowSession{
		name:       name,
		directory:  directory,
		entry:      entry,
		language:   strings.ToLower(language),
		dialect:    dialect,
		pythonPath: pythonPath,
		nodePath:   nodePath,
		clientDir:  clientDir,
		bridge:     bridge,
		pending:    map[string]chan flowInbound{},
	}
}

// Start launches a long-lived process. One-shot plugins do not start until the first call.
func (s *flowSession) Start(ctx context.Context) error {
	if s.dialect != flowDialectV2 {
		return nil
	}
	s.mu.Lock()
	err := s.ensureV2Locked()
	done := s.done
	s.mu.Unlock()
	if err != nil {
		return err
	}
	select {
	case <-done:
		return s.currentExitError()
	default:
		return nil
	}
}

// Invoke performs one plugin call. A cancelled context kills a long-lived process
// that does not answer, so the next call can start clean.
func (s *flowSession) Invoke(ctx context.Context, method string, params []any, settings map[string]any) (flowReply, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s.dialect == flowDialectV1 {
		return s.invokeV1(ctx, method, params, settings)
	}
	reply, err := s.invokeV2(ctx, method, params, settings)
	if err == nil || s.everV2 || !errors.Is(err, errFlowProcessExited) || ctx.Err() != nil {
		return reply, err
	}
	// The process left before speaking JSON-RPC. Retry once with the argv protocol
	// used by clients that never read stdin. Stay on the long-lived protocol when
	// that retry fails, so a crashed plugin is not pinned to the wrong dialect.
	util.GetLogger().Info(ctx, fmt.Sprintf("[flow-jsonrpc:%s] process exited before a JSON-RPC response, retrying once as a one-shot call", s.name))
	oneShotReply, oneShotErr := s.invokeV1(ctx, method, params, settings)
	if oneShotErr != nil {
		return flowReply{}, err
	}
	s.mu.Lock()
	s.dialect = flowDialectV1
	s.mu.Unlock()
	return oneShotReply, nil
}

// Close stops the long-lived process. A later Invoke fails.
func (s *flowSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.killLocked()
}

func (s *flowSession) invokeV2(ctx context.Context, method string, params []any, settings map[string]any) (flowReply, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, flowCallTimeout)
		defer cancel()
	}

	s.mu.Lock()
	if err := s.ensureV2Locked(); err != nil {
		s.mu.Unlock()
		return flowReply{}, err
	}
	s.nextID++
	id := fmt.Sprintf("wox-%d", s.nextID)
	wait := make(chan flowInbound, 1)
	s.pending[id] = wait
	s.callCtx = ctx
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
	}()

	request := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	if settings != nil && method == "query" {
		request["params"] = append(append([]any{}, params...), settings)
	}
	if err := s.writeJSON(request); err != nil {
		return flowReply{}, err
	}

	select {
	case message := <-wait:
		return s.finishV2(message)
	case <-ctx.Done():
		timer := time.NewTimer(200 * time.Millisecond)
		defer timer.Stop()
		select {
		case message := <-wait:
			return s.finishV2(message)
		case <-timer.C:
			s.mu.Lock()
			s.killLocked()
			s.mu.Unlock()
			return flowReply{}, ctx.Err()
		}
	}
}

func (s *flowSession) finishV2(message flowInbound) (flowReply, error) {
	if message.exited {
		return flowReply{}, fmt.Errorf("%w: %s", errFlowProcessExited, s.currentExitError())
	}
	if message.err != nil {
		return flowReply{}, message.err
	}
	s.mu.Lock()
	s.everV2 = true
	s.mu.Unlock()
	return message.reply, nil
}

func (s *flowSession) ensureV2Locked() error {
	if s.closed {
		return errors.New("plugin process is closed")
	}
	if s.alive && s.cmd != nil {
		return nil
	}
	return s.startV2Locked()
}

func (s *flowSession) startV2Locked() error {
	program, args, env, err := s.commandSetup(false, "")
	if err != nil {
		return err
	}
	cmd := shell.BuildCommand(program, nil, args...)
	cmd.Dir = s.directory
	cmd.Env = env
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	s.cmd = cmd
	s.stdin = stdin
	s.alive = true
	s.exitErr = nil
	s.stderr.Reset()
	s.done = make(chan struct{})
	done := s.done
	go s.readStderr(stderr)
	go s.readStdout(stdout, cmd, stdin, done)
	util.GetLogger().Info(context.Background(), fmt.Sprintf("[flow-jsonrpc:%s] started %s", s.name, program))
	return nil
}

func (s *flowSession) readStdout(stdout io.Reader, cmd *exec.Cmd, stdin io.WriteCloser, done chan struct{}) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		s.dispatchLine(scanner.Text())
	}
	waitErr := cmd.Wait()
	s.mu.Lock()
	// A restart may already have replaced this process. Only clear the fields
	// that still point at the process that just exited.
	if s.cmd == cmd {
		s.alive = false
		s.cmd = nil
		if s.stdin == stdin {
			_ = s.stdin.Close()
			s.stdin = nil
		}
		if waitErr != nil && s.exitErr == nil {
			s.exitErr = waitErr
		}
		if s.stderr.Len() > 0 && s.exitErr == nil {
			s.exitErr = errors.New(strings.TrimSpace(s.stderr.String()))
		}
		for id, wait := range s.pending {
			select {
			case wait <- flowInbound{id: id, exited: true}:
			default:
			}
		}
	}
	s.mu.Unlock()
	close(done)
}

func (s *flowSession) readStderr(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 16*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		s.mu.Lock()
		if s.stderr.Len() < 4096 {
			s.stderr.WriteString(line)
			s.stderr.WriteByte('\n')
		}
		s.mu.Unlock()
		util.GetLogger().Info(context.Background(), fmt.Sprintf("[flow-jsonrpc:%s] %s", s.name, line))
	}
}

func (s *flowSession) dispatchLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	var document map[string]any
	if err := json.Unmarshal([]byte(line), &document); err != nil {
		util.GetLogger().Debug(context.Background(), fmt.Sprintf("[flow-jsonrpc:%s] ignored non-JSON output", s.name))
		return
	}
	method := flowObjectString(document, "method")
	id, hasID := flowRPCID(document["id"])
	if method != "" && !strings.HasPrefix(id, "wox-") {
		params := flowObjectParams(document)
		s.mu.Lock()
		callCtx := s.callCtx
		s.mu.Unlock()
		if callCtx == nil {
			callCtx = context.Background()
		}
		handleFlowMethod(callCtx, s.bridge, method, params)
		if hasID {
			_ = s.writeJSON(map[string]any{"jsonrpc": "2.0", "id": document["id"], "result": nil})
		}
		return
	}
	if !hasID {
		return
	}
	reply, err := flowReplyFromValue(document)
	message := flowInbound{id: id, reply: reply, err: err}
	s.mu.Lock()
	wait := s.pending[id]
	s.mu.Unlock()
	if wait == nil {
		return
	}
	select {
	case wait <- message:
	default:
	}
}

func (s *flowSession) invokeV1(ctx context.Context, method string, params []any, settings map[string]any) (flowReply, error) {
	payload := map[string]any{
		"method":     method,
		"parameters": flowV1CallParameters(method, params),
		"settings":   settings,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return flowReply{}, err
	}
	program, args, env, err := s.commandSetup(true, string(raw))
	if err != nil {
		return flowReply{}, err
	}
	cmd := shell.BuildCommand(program, nil, args...)
	cmd.Dir = s.directory
	cmd.Env = env
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return flowReply{}, err
	}
	wait := make(chan error, 1)
	go func() { wait <- cmd.Wait() }()
	select {
	case err := <-wait:
		if text := strings.TrimSpace(stderr.String()); text != "" {
			util.GetLogger().Info(ctx, fmt.Sprintf("[flow-jsonrpc:%s] %s", s.name, text))
		}
		reply, parseErr := parseFlowOutput(ctx, s.bridge, stdout.String())
		if parseErr != nil {
			return flowReply{}, parseErr
		}
		if err != nil && len(reply.Results) == 0 && !reply.HasSettings && strings.TrimSpace(stdout.String()) == "" {
			if text := strings.TrimSpace(stderr.String()); text != "" {
				return flowReply{}, fmt.Errorf("%w: %s", err, text)
			}
			return flowReply{}, err
		}
		return reply, nil
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return flowReply{}, ctx.Err()
	}
}

func (s *flowSession) commandSetup(oneShot bool, payload string) (string, []string, []string, error) {
	entry, err := flowPathInsideHost(s.directory, s.entry)
	if err != nil {
		return "", nil, nil, err
	}
	env := map[string]string{
		"PYTHONIOENCODING": "utf-8",
		"PYTHONUNBUFFERED": "1",
	}
	var program string
	var args []string
	switch s.language {
	case "python":
		if s.pythonPath == "" {
			return "", nil, nil, errors.New("Python was not found")
		}
		program = s.pythonPath
		args = []string{"-B", "-u", entry}
		if !oneShot && s.clientDir != "" {
			env["PYTHONPATH"] = prependPathEntry(s.clientDir, os.Getenv("PYTHONPATH"))
		}
	case "javascript", "typescript":
		if s.nodePath == "" {
			return "", nil, nil, errors.New("Node.js was not found")
		}
		program = s.nodePath
		args = []string{entry}
	case "executable":
		program = entry
	default:
		return "", nil, nil, fmt.Errorf("unsupported plugin language %s", s.language)
	}
	if oneShot {
		args = append(args, payload)
	}
	return program, args, mergeProcessEnv(env), nil
}

func (s *flowSession) writeJSON(value any) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.stdin == nil {
		return errors.New("plugin stdin is closed")
	}
	encoder := json.NewEncoder(s.stdin)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func (s *flowSession) killLocked() {
	s.alive = false
	if s.stdin != nil {
		_ = s.stdin.Close()
		s.stdin = nil
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	for id, wait := range s.pending {
		select {
		case wait <- flowInbound{id: id, exited: true}:
		default:
		}
	}
}

func (s *flowSession) currentExitError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.exitErr != nil {
		return s.exitErr
	}
	if text := strings.TrimSpace(s.stderr.String()); text != "" {
		return errors.New(text)
	}
	return errFlowProcessExited
}

// flowV1CallParameters matches the argv protocol. A query's first parameter is the
// search text, because those plugins read parameters[0] as a string. A query object
// would stringify to "[object Object]" or fail the length check and look like an
// empty query. Action parameters stay as the plugin returned them. The long-lived
// protocol still receives the query object.
func flowV1CallParameters(method string, params []any) []any {
	if method != "query" || len(params) == 0 {
		return params
	}
	search, ok := flowQuerySearchText(params[0])
	if !ok {
		return params
	}
	rewritten := make([]any, len(params))
	copy(rewritten, params)
	rewritten[0] = search
	return rewritten
}

func flowQuerySearchText(value any) (string, bool) {
	object, ok := value.(map[string]any)
	if !ok {
		return "", false
	}
	raw, exists := object["Search"]
	if !exists {
		raw, exists = object["search"]
	}
	if !exists {
		return "", false
	}
	switch typed := raw.(type) {
	case string:
		return typed, true
	case nil:
		return "", true
	default:
		return fmt.Sprint(typed), true
	}
}

func flowRPCID(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return "", false
		}
		return typed, true
	case float64:
		return fmt.Sprintf("%d", int64(typed)), true
	case json.Number:
		return typed.String(), true
	default:
		return "", false
	}
}

func flowPathInsideHost(directory, name string) (string, error) {
	if filepath.IsAbs(name) {
		return name, nil
	}
	cleaned := filepath.Clean(filepath.Join(directory, filepath.FromSlash(name)))
	relative, err := filepath.Rel(filepath.Clean(directory), cleaned)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("entry %s escapes the plugin directory", name)
	}
	return cleaned, nil
}

func prependPathEntry(entry, current string) string {
	if strings.TrimSpace(current) == "" {
		return entry
	}
	return entry + string(os.PathListSeparator) + current
}

func mergeProcessEnv(overrides map[string]string) []string {
	skip := map[string]bool{}
	merged := make([]string, 0, len(os.Environ())+len(overrides))
	for key, value := range overrides {
		merged = append(merged, key+"="+value)
		skip[envKey(key)] = true
	}
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if skip[envKey(key)] {
			continue
		}
		merged = append(merged, item)
	}
	return merged
}

func envKey(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}

// materializeFlowClient writes the host-provided Python client into dir/flowlauncher.
func materializeFlowClient(dir string) error {
	target := filepath.Join(dir, "flowlauncher")
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(target, "__init__.py"), []byte(flowLauncherClientSource), 0o644)
}
