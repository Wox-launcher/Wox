package extension

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"wox/plugin"
	"wox/util"
	"wox/util/shell"
)

type runtimeMessage struct {
	Type     string                `json:"type"`
	ID       string                `json:"id"`
	Method   string                `json:"method"`
	Params   json.RawMessage       `json:"params"`
	Response *plugin.QueryResponse `json:"response"`
	Error    string                `json:"error"`
	Text     string                `json:"text"`
	Level    string                `json:"level"`
}

// commandSession is an isolated process for one command in one launcher window.
// It retains a query snapshot, not a UI tree. Contexts keep the window identity after a query is canceled.
type commandSession struct {
	mu        sync.Mutex
	writeMu   sync.Mutex
	command   Command
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	done      chan struct{}
	closed    bool
	nextID    int
	pending   map[string]chan runtimeMessage
	ctx       context.Context
	logCtx    context.Context
	query     plugin.Query
	response  plugin.QueryResponse
	published []byte
	errorText string
	timer     *time.Timer
	prefix    string
	owner     *extensionPlugin
}

// startSession keeps the process lifetime independent of any single search request.
func (p *extensionPlugin) startSession(ctx context.Context, query plugin.Query, command Command) (*commandSession, error) {
	cmd := exec.Command(p.node, p.bundle)
	cmd.Dir = p.descriptor.Metadata.Directory
	shell.HideWindowCmd(cmd)
	shell.PrepareLifetimeBoundCmd(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, err
	}
	s := &commandSession{command: command, cmd: cmd, stdin: stdin, done: make(chan struct{}), pending: map[string]chan runtimeMessage{},
		ctx: context.WithoutCancel(ctx), logCtx: context.WithoutCancel(ctx), query: query, owner: p, prefix: util.GetContextTraceId(util.NewTraceContext()) + ":"}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, err
	}
	if err := shell.AdoptLifetimeBoundCmd(ctx, cmd); err != nil {
		_ = cmd.Wait()
		return nil, err
	}
	go s.read(stdout)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			util.GetLogger().Warn(s.logCtx, "[Raycast] "+scanner.Text())
		}
	}()
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		if !s.closed && command.Mode != "no-view" {
			s.errorText = fmt.Sprintf("Raycast command exited: %v", err)
			s.scheduleLocked()
		}
		s.mu.Unlock()
		close(s.done)
	}()
	return s, nil
}

// read services API calls independently from replies to avoid blocking a command on its own callback.
func (s *commandSession) read(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 8<<20)
	for scanner.Scan() {
		var message runtimeMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			util.GetLogger().Warn(s.logCtx, "Raycast emitted an invalid runtime message")
			continue
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return
		}
		switch message.Type {
		case "reply":
			if wait := s.pending[message.ID]; wait != nil {
				select {
				case wait <- message:
				default:
				}
			}
		case "render":
			if message.Response != nil {
				s.response = *message.Response
				s.scheduleLocked()
			}
		case "error":
			s.errorText = message.Error
			s.scheduleLocked()
		case "log":
			util.GetLogger().Info(s.logCtx, "[Raycast] "+message.Text)
		case "api":
			go s.answerAPI(message)
		}
		s.mu.Unlock()
	}
	if err := scanner.Err(); err != nil {
		util.GetLogger().Warn(s.logCtx, fmt.Sprintf("Raycast protocol read: %v", err))
		s.close()
	}
}

func (s *commandSession) scheduleLocked() {
	if s.command.Mode == "no-view" {
		return
	}
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(40*time.Millisecond, func() { s.owner.publish(s) })
}

// call correlates replies while asynchronous React commits and host API calls continue flowing.
func (s *commandSession) call(ctx context.Context, method string, params any) (runtimeMessage, error) {
	requestCtx := ctx
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return runtimeMessage{}, fmt.Errorf("Raycast session is closed")
	}
	s.nextID++
	id := fmt.Sprint(s.nextID)
	wait := make(chan runtimeMessage, 1)
	s.pending[id] = wait
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.pending, id); s.mu.Unlock() }()
	if err := s.write(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		return runtimeMessage{}, err
	}
	select {
	case reply := <-wait:
		if reply.Error != "" {
			return reply, fmt.Errorf("%s", reply.Error)
		}
		return reply, nil
	case <-ctx.Done():
		if requestCtx.Err() == nil {
			// A canceled search is normal; an unresponsive command must not keep consuming resources.
			s.close()
		}
		return runtimeMessage{}, ctx.Err()
	case <-s.done:
		return runtimeMessage{}, fmt.Errorf("Raycast command exited")
	}
}

func (s *commandSession) write(value any) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return json.NewEncoder(s.stdin).Encode(value)
}

// close invalidates late commits before terminating the process and releasing its pipes.
func (s *commandSession) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	if s.timer != nil {
		s.timer.Stop()
	}
	s.mu.Unlock()
	// Give React effect cleanup a bounded opportunity to finish, even when command JS is stuck.
	go func() { _ = s.write(map[string]any{"id": "stop", "method": "stop"}) }()
	go func() {
		select {
		case <-s.done:
		case <-time.After(250 * time.Millisecond):
			_ = s.cmd.Process.Kill()
		}
		_ = s.stdin.Close()
	}()
}
