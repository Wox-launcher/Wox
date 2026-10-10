package supervisor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

const controlRetryWindow = 10 * time.Second

type controlRequest struct {
	ID   string `json:"id,omitempty"`
	Op   string `json:"op"`
	Task *Task  `json:"task,omitempty"`
}

type controlResponse struct {
	ID     string      `json:"id,omitempty"`
	OK     bool        `json:"ok"`
	Error  string      `json:"error,omitempty"`
	Result *TaskResult `json:"result,omitempty"`
}

// Submit sends a task to the running supervisor. When this process is not
// already supervised, it starts one and waits until the connection accepts the task.
func Submit(ctx context.Context, task Task) error {
	if err := submitOnce(task); err == nil {
		return nil
	} else if GetManager().IsChildArg(os.Args) {
		return err
	}
	if err := GetManager().StartSupervisorDetached(ctx, true); err != nil {
		return err
	}
	deadline := time.Now().Add(controlRetryWindow)
	var last error
	for time.Now().Before(deadline) {
		last = submitOnce(task)
		if last == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if last == nil {
		last = fmt.Errorf("supervisor did not accept the task")
	}
	return last
}

// TakeTaskResult reads the outcome of the task that ran before this process started.
func TakeTaskResult() (TaskResult, bool) {
	response, err := roundTrip(controlRequest{Op: "takeResult"})
	if err != nil || response.Result == nil {
		return TaskResult{}, false
	}
	return *response.Result, true
}

func submitOnce(task Task) error {
	response, err := roundTrip(controlRequest{Op: "submit", Task: &task})
	if err != nil {
		return err
	}
	if !response.OK {
		if response.Error == "" {
			return fmt.Errorf("supervisor rejected the task")
		}
		return fmt.Errorf("%s", response.Error)
	}
	return nil
}

func roundTrip(request controlRequest) (controlResponse, error) {
	conn, err := dialControl()
	if err != nil {
		return controlResponse{}, err
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return controlResponse{}, err
	}
	var response controlResponse
	if err := json.NewDecoder(io.LimitReader(conn, 1<<20)).Decode(&response); err != nil {
		return controlResponse{}, err
	}
	return response, nil
}

// serveControl handles requests on the endpoint owned by this supervisor.
func (m *Manager) serveControl(ctx context.Context, listener controlListener) {
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		m.handleControl(conn)
		_ = conn.Close()
	}
}

func (m *Manager) handleControl(conn io.ReadWriteCloser) {
	var request controlRequest
	if err := json.NewDecoder(io.LimitReader(conn, 1<<20)).Decode(&request); err != nil {
		_ = json.NewEncoder(conn).Encode(controlResponse{OK: false, Error: err.Error()})
		return
	}
	response := controlResponse{ID: request.ID, OK: true}
	switch request.Op {
	case "submit":
		if request.Task == nil {
			response.OK = false
			response.Error = "task is required"
			break
		}
		if err := m.acceptTask(*request.Task); err != nil {
			response.OK = false
			response.Error = err.Error()
		}
	case "takeResult":
		if result, ok := m.takeTaskResult(); ok {
			response.Result = &result
		}
	default:
		response.OK = false
		response.Error = "unknown operation"
	}
	_ = json.NewEncoder(conn).Encode(response)
}
