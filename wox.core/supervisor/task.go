package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

const (
	// TaskRestore replaces the user data directory after Wox has exited.
	TaskRestore = "restore"
	// RunAfterExit runs a task only once the Wox process and its children are gone.
	RunAfterExit = "exit"
)

// Task is one unit of work the running Wox process asks the supervisor to perform.
type Task struct {
	Type     string          `json:"type"`
	RunAfter string          `json:"runAfter"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

// TaskResult is the outcome the next Wox process reads after it starts.
type TaskResult struct {
	Type  string `json:"type"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// Handler performs one task type. Registration lives outside this package so
// the supervisor does not depend on settings, plugins, or the UI.
type Handler func(ctx context.Context, payload json.RawMessage) error

var (
	handlerMu sync.Mutex
	handlers  = map[string]Handler{}

	taskMu     sync.Mutex
	pending    *Task
	lastResult *TaskResult
)

// Register binds a task type to the function that performs it.
func Register(taskType string, handler Handler) {
	handlerMu.Lock()
	handlers[taskType] = handler
	handlerMu.Unlock()
}

func handlerFor(taskType string) (Handler, bool) {
	handlerMu.Lock()
	handler, ok := handlers[taskType]
	handlerMu.Unlock()
	return handler, ok
}

func (m *Manager) taskJournalPath() string {
	return m.RuntimeDirectory() + string(os.PathSeparator) + "task.json"
}

// acceptTask records one after-exit task. A second task is rejected until it runs.
func (m *Manager) acceptTask(task Task) error {
	if task.Type == "" {
		return fmt.Errorf("task type is required")
	}
	if task.RunAfter != RunAfterExit {
		return fmt.Errorf("unsupported task timing %q", task.RunAfter)
	}
	taskMu.Lock()
	defer taskMu.Unlock()
	if pending != nil {
		return fmt.Errorf("a task is already waiting")
	}
	if err := m.writeTaskJournal(task); err != nil {
		return err
	}
	accepted := task
	pending = &accepted
	m.logf("accepted task %s", task.Type)
	return nil
}

func (m *Manager) loadTaskJournal() {
	data, err := os.ReadFile(m.taskJournalPath())
	if err != nil {
		return
	}
	var task Task
	if json.Unmarshal(data, &task) != nil || task.Type == "" {
		_ = os.Remove(m.taskJournalPath())
		return
	}
	taskMu.Lock()
	pending = &task
	taskMu.Unlock()
}

func (m *Manager) writeTaskJournal(task Task) error {
	if err := os.MkdirAll(m.RuntimeDirectory(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomicFile(m.taskJournalPath(), append(data, '\n'))
}

func (m *Manager) clearTaskJournal() {
	_ = os.Remove(m.taskJournalPath())
}

func (m *Manager) hasPendingTask() bool {
	taskMu.Lock()
	defer taskMu.Unlock()
	return pending != nil
}

// runPendingTask performs the accepted task and keeps the outcome for the next launch.
func (m *Manager) runPendingTask(ctx context.Context) bool {
	taskMu.Lock()
	task := pending
	pending = nil
	taskMu.Unlock()
	if task == nil {
		return false
	}
	m.clearTaskJournal()

	result := TaskResult{Type: task.Type, OK: true}
	handler, ok := handlerFor(task.Type)
	if !ok {
		result.OK = false
		result.Error = fmt.Sprintf("no handler for task %s", task.Type)
	} else if err := handler(ctx, task.Payload); err != nil {
		result.OK = false
		result.Error = err.Error()
	}
	if result.OK {
		m.logf("task %s completed", task.Type)
	} else {
		m.logf("task %s failed: %s", task.Type, result.Error)
	}
	taskMu.Lock()
	lastResult = &result
	taskMu.Unlock()
	return true
}

// takeTaskResult returns the latest task outcome once.
func (m *Manager) takeTaskResult() (TaskResult, bool) {
	taskMu.Lock()
	defer taskMu.Unlock()
	if lastResult == nil {
		return TaskResult{}, false
	}
	result := *lastResult
	lastResult = nil
	return result, true
}

func writeAtomicFile(path string, data []byte) error {
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0644); err != nil {
		return err
	}
	_ = os.Remove(path)
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}
