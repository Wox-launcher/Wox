package host

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// hostProcess has one Wait owner; closing done publishes the exit result after stderr is drained.
type hostProcess struct {
	cmd    *exec.Cmd
	done   chan struct{}
	err    error
	stderr *hostStderr
}

// watchHostProcess cancels transport retries when the child exits, including a successful early exit.
func watchHostProcess(cmd *exec.Cmd, stderr *hostStderr, cancel context.CancelCauseFunc) *hostProcess {
	process := &hostProcess{cmd: cmd, done: make(chan struct{}), stderr: stderr}
	// Descendants can inherit stderr. Do not let an inherited pipe hide the host's exit indefinitely.
	cmd.WaitDelay = time.Second
	go func() {
		process.err = cmd.Wait()
		close(process.done)
		cancel(process.exitError())
	}()
	return process
}

// exitError is read only after Wait completes and prioritizes the diagnostic in compact UI surfaces.
func (p *hostProcess) exitError() error {
	message := fmt.Sprintf("host process exited (code %d)", p.cmd.ProcessState.ExitCode())
	if p.err != nil {
		message += ": " + p.err.Error()
	}
	if detail := p.stderr.detail(); detail != "" {
		// Runtime cards reserve two lines, so put the actionable cause before process metadata.
		message = detail + " (" + message + ")"
	}
	return fmt.Errorf("%s", message)
}

// hostStderr retains only a bounded tail; full output is independently sent to the Wox logger.
type hostStderr struct {
	mu   sync.Mutex
	tail []byte
}

// Write drops old bytes without blocking the child's stderr on UI consumption.
func (s *hostStderr) Write(data []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	const limit = 16 * 1024
	size := len(data)
	if len(data) >= limit {
		s.tail = append(s.tail[:0], data[len(data)-limit:]...)
	} else {
		if excess := len(s.tail) + len(data) - limit; excess > 0 {
			s.tail = s.tail[excess:]
		}
		s.tail = append(s.tail, data...)
	}
	return size, nil
}

// detail favors exception messages over minified source and traceback frames in compact settings cards.
func (s *hostStderr) detail() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	lines := strings.Split(strings.ToValidUTF8(string(s.tail), "�"), "\n")
	detail := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		detail = line
		if strings.Contains(strings.ToLower(line), "error") && len(line) < 1024 {
			break
		}
	}
	const limit = 600
	if runes := []rune(detail); len(runes) > limit {
		detail = string(runes[:limit]) + "…"
	}
	return detail
}
