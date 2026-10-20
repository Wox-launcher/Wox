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
	Loading  bool                  `json:"loading"`
	Error    string                `json:"error"`
	Text     string                `json:"text"`
	Level    string                `json:"level"`
}

// commandSession is an isolated process for one command in one launcher window.
// It retains a query snapshot, not a UI tree. Contexts keep the window identity after a query is canceled.
type commandSession struct {
	mu      sync.Mutex
	writeMu sync.Mutex
	// Serialize async row publication with confirmation display/restoration.
	displayMu           sync.Mutex
	command             Command
	cmd                 *exec.Cmd
	stdin               io.WriteCloser
	done                chan struct{}
	closed              bool
	nextID              int
	pending             map[string]chan runtimeMessage
	ctx                 context.Context
	logCtx              context.Context
	query               plugin.Query
	response            plugin.QueryResponse
	loading             bool
	changed             chan struct{}
	published           []byte
	errorText           string
	timer               *time.Timer
	prefix              string
	owner               *extensionPlugin
	actionResultID      string
	confirmation        *resultConfirmation
	confirmationChanged chan struct{}
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
	s := &commandSession{command: command, cmd: cmd, stdin: stdin, done: make(chan struct{}), changed: make(chan struct{}, 1), confirmationChanged: make(chan struct{}, 1), pending: map[string]chan runtimeMessage{},
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
		// Apply snapshots in wire order; a query reply must not overwrite a newer async render.
		if message.Response != nil && (message.Type == "reply" || message.Type == "render") {
			s.response, s.loading = *message.Response, message.Loading
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
		select {
		case s.changed <- struct{}{}:
		default:
		}
		s.mu.Unlock()
	}
	if err := scanner.Err(); err != nil {
		util.GetLogger().Warn(s.logCtx, fmt.Sprintf("Raycast protocol read: %v", err))
		s.close()
	}
}

// waitForResults leaves an empty, loading view pending so Wox owns the loading animation.
// Waiting in Go keeps Node's request queue available when a newer search cancels this query.
func (s *commandSession) waitForResults(ctx context.Context) error {
	requestCtx := ctx
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for {
		s.mu.Lock()
		closed := s.closed
		waiting := s.loading && len(s.response.Results) == 0 && s.errorText == ""
		s.mu.Unlock()
		if closed {
			return fmt.Errorf("Raycast session is closed")
		}
		if !waiting {
			return nil
		}
		select {
		case <-s.changed:
		case <-s.done:
			return fmt.Errorf("Raycast command exited")
		case <-ctx.Done():
			if requestCtx.Err() == nil {
				s.close()
			}
			return ctx.Err()
		}
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
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
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
	for {
		select {
		case reply := <-wait:
			if reply.Error != "" {
				return reply, fmt.Errorf("%s", reply.Error)
			}
			return reply, nil
		case <-ctx.Done():
			return runtimeMessage{}, ctx.Err()
		case <-s.confirmationChanged:
			s.mu.Lock()
			waiting := s.confirmation != nil
			s.mu.Unlock()
			if method == "action" && waiting {
				timer.Stop()
			} else {
				timer.Reset(30 * time.Second)
			}
		case <-timer.C:
			s.mu.Lock()
			waiting := method == "action" && s.confirmation != nil
			s.mu.Unlock()
			if waiting {
				continue
			}
			s.close()
			return runtimeMessage{}, context.DeadlineExceeded
		case <-s.done:
			return runtimeMessage{}, fmt.Errorf("Raycast command exited")
		}
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
	if s.confirmation != nil {
		s.confirmation.answer <- false
		s.confirmation = nil
	}
	select {
	case s.changed <- struct{}{}:
	default:
	}
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
