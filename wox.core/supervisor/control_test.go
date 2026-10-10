package supervisor

import (
	"context"
	"encoding/json"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestProductionFilesDoNotImportWox(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `"wox/`) {
			t.Errorf("%s imports another Wox package", name)
		}
	}
}

func TestControlSubmitRunsAfterExit(t *testing.T) {
	setupSupervisorTest(t)
	startTestControl(t)

	ran := ""
	Register("marker", func(_ context.Context, payload json.RawMessage) error {
		ran = string(payload)
		return nil
	})
	task := Task{Type: "marker", RunAfter: RunAfterExit, Payload: json.RawMessage(`"ok"`)}
	deadline := time.Now().Add(3 * time.Second)
	var submitErr error
	for time.Now().Before(deadline) {
		submitErr = submitOnce(task)
		if submitErr == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if submitErr != nil {
		t.Fatal(submitErr)
	}
	if !GetManager().runPendingTask(context.Background()) {
		t.Fatal("accepted task should run")
	}
	if ran != `"ok"` {
		t.Fatalf("handler payload = %s", ran)
	}
	result, ok := TakeTaskResult()
	if !ok || !result.OK || result.Type != "marker" {
		t.Fatalf("result = %+v, ok=%t", result, ok)
	}
}

// setupSupervisorTest isolates process-wide state and the endpoint inherited by test children.
func setupSupervisorTest(t *testing.T) string {
	t.Helper()
	var directory string
	if runtime.GOOS == "windows" {
		directory = t.TempDir()
	} else {
		// Unix sockets have a short path limit; macOS's default temp directory
		// plus a descriptive test name can exceed it before the socket suffix.
		var err error
		directory, err = os.MkdirTemp("/tmp", "wox-supervisor-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.RemoveAll(directory); err != nil {
				t.Error(err)
			}
		})
	}
	SetDataDirectory(directory)
	SetControlAddress("")
	t.Setenv(controlAddressEnv, newControlAddress())
	t.Cleanup(func() {
		SetDataDirectory("")
		SetControlAddress("")
		taskMu.Lock()
		pending = nil
		lastResult = nil
		taskMu.Unlock()
	})
	return directory
}

// startTestControl waits for shutdown before test cleanup resets shared paths.
func startTestControl(t *testing.T) func() {
	t.Helper()
	listener, err := listenControl()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		GetManager().serveControl(ctx, listener)
	}()
	stop := func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("control server did not stop")
		}
	}
	t.Cleanup(stop)
	return stop
}

func TestControlRestartKeepsNewEndpoint(t *testing.T) {
	setupSupervisorTest(t)
	oldAddress := controlAddress()
	stopOld := startTestControl(t)

	newAddress := newControlAddress()
	if newAddress == oldAddress {
		t.Fatal("restart reused the old endpoint")
	}
	SetControlAddress(newAddress)
	startTestControl(t)
	stopOld()

	var response controlResponse
	var err error
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		response, err = roundTrip(controlRequest{ID: "new-supervisor", Op: "takeResult"})
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err != nil || !response.OK || response.ID != "new-supervisor" {
		t.Fatalf("new endpoint after old listener closed: response=%+v err=%v", response, err)
	}
}

func TestControlAddressInheritsSupervisorEndpoint(t *testing.T) {
	setupSupervisorTest(t)
	address := controlAddress()
	SetControlAddress(newControlAddress())
	SetControlAddress("")
	if got := controlAddress(); got != address {
		t.Fatalf("inherited address = %q, want %q", got, address)
	}
}
