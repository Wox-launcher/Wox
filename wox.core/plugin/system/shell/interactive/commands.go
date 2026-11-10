package interactive

import (
	"bytes"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	FocusActionID   = "focus-interactive-shell"
	HistoryIDPrefix = "interactive:"
)

type CommandStatus uint8

const (
	CommandRunning CommandStatus = iota
	CommandCompleted
	CommandFailed
)

// HistoryEntry describes the single result for an interactive shell.
type HistoryEntry struct {
	ID        string
	Title     string
	Status    CommandStatus
	StartedAt time.Time
	Saved     bool
}

// commandTracker consumes shell-reported boundaries without inspecting terminal input.
type commandTracker struct {
	prefix   string
	pending  []byte
	command  string
	history  HistoryEntry
	onStart  func()
	onFinish func(HistoryEntry, string, int) bool
}

// observe writes the bytes before each marker so capture uses the cursor at that boundary.
func (t *commandTracker) observe(output []byte, write func([]byte) bool) bool {
	const maxPayload = 16 * 1024
	stream := output
	if len(t.pending) != 0 {
		stream = append(t.pending, output...)
		t.pending = nil
	}
	prefix := []byte(t.prefix)
	for {
		index := bytes.Index(stream, prefix)
		if index < 0 {
			tail := max(0, len(stream)-len(prefix)+1)
			if i := bytes.LastIndexByte(stream[tail:], '\x1b'); i >= 0 && bytes.HasPrefix(prefix, stream[tail+i:]) {
				t.pending = bytes.Clone(stream[tail+i:])
				stream = stream[:tail+i]
			}
			return write(stream)
		}
		if !write(stream[:index]) {
			return false
		}
		body := stream[index+len(prefix):]
		end, terminator := bytes.IndexByte(body, '\a'), 1
		if st := bytes.Index(body, []byte("\x1b\\")); st >= 0 && (end < 0 || st < end) {
			end, terminator = st, 2
		}
		if end < 0 {
			if len(body) <= maxPayload {
				t.pending = bytes.Clone(stream[index:])
				return true
			}
			return write(stream[index:])
		}
		if end <= maxPayload {
			kind, value, _ := strings.Cut(string(body[:end]), ";")
			switch kind {
			case "start":
				if text := strings.TrimSpace(value); text != "" && strings.IndexFunc(text, unicode.IsControl) < 0 {
					t.finish(-1)
					t.command = text
					if t.history.Title == "" {
						t.history.Title = text
						t.history.StartedAt = time.Now()
					}
					t.history.Status = CommandRunning
					if t.onStart != nil {
						t.onStart()
					}
				}
			case "end":
				if status, err := strconv.Atoi(value); err == nil {
					t.finish(status)
				}
			}
		}
		stream = body[end+terminator:]
	}
}

func (t *commandTracker) finish(exitCode int) {
	if t.command == "" {
		return
	}
	t.history.Status = CommandFailed
	if exitCode == 0 {
		t.history.Status = CommandCompleted
	}
	if t.onFinish != nil && t.onFinish(t.history, t.command, exitCode) {
		t.history.Saved = true
	}
	t.command = ""
}
