package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

const (
	supervisorTestModeEnv = "WOX_SUPERVISOR_TEST_MODE"
	supervisorTestDirEnv  = "WOX_SUPERVISOR_TEST_DIRECTORY"
	shutdownTestTask      = "test-shutdown-restore"
)

// TestMain lets the real supervisor launch this test binary as a Wox child.
func TestMain(m *testing.M) {
	mode := os.Getenv(supervisorTestModeEnv)
	if mode != "" && GetManager().IsChildArg(os.Args) {
		if mode == "crash-with-task" {
			directory := os.Getenv(supervisorTestDirEnv)
			SetDataDirectory(directory)
			if _, err := os.Stat(filepath.Join(directory, "task-submitted")); os.IsNotExist(err) {
				if err := Submit(context.Background(), Task{Type: shutdownTestTask, RunAfter: RunAfterExit}); err != nil {
					os.Exit(8)
				}
				if err := os.WriteFile(filepath.Join(directory, "task-submitted"), []byte("accepted"), 0644); err != nil {
					os.Exit(9)
				}
				os.Exit(7)
			}
			result, ok := TakeTaskResult()
			if !ok {
				os.Exit(10)
			}
			data, err := json.Marshal(result)
			if err != nil || os.WriteFile(filepath.Join(directory, "child-result.json"), data, 0644) != nil {
				os.Exit(11)
			}
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestWaitForParentExitRejectsLiveParent(t *testing.T) {
	setupSupervisorTest(t)
	if err := GetManager().waitForParentExit(context.Background(), os.Getpid(), 0); err == nil {
		t.Fatal("live parent was treated as exited")
	}
}

func TestWaitForParentExitAcceptsExitedParent(t *testing.T) {
	setupSupervisorTest(t)
	t.Setenv(supervisorTestModeEnv, "exit")
	cmd := exec.Command(os.Args[0], ArgChild)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	if err := GetManager().waitForParentExit(context.Background(), cmd.Process.Pid, 0); err != nil {
		t.Fatalf("exited parent was rejected: %v", err)
	}
}

func TestSupervisorRetainsTaskWhenParentExitIsNotConfirmed(t *testing.T) {
	setupSupervisorTest(t)
	m := GetManager()
	ran := false
	Register("test-parent-wait", func(context.Context, json.RawMessage) error {
		ran = true
		return nil
	})
	if err := m.acceptTask(Task{Type: "test-parent-wait", RunAfter: RunAfterExit}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	args := []string{os.Args[0], ArgSupervisor, ArgWaitParent, strconv.Itoa(os.Getpid())}
	if code := m.RunSupervisor(ctx, args); code != 1 {
		t.Fatalf("unconfirmed parent exit returned code %d", code)
	}
	if ran || !m.hasPendingTask() {
		t.Fatalf("task ran=%t pending=%t", ran, m.hasPendingTask())
	}
	if _, err := os.Stat(m.taskJournalPath()); err != nil {
		t.Fatalf("accepted task journal was lost: %v", err)
	}
	if state := m.LoadState(); state.ChildPid != 0 {
		t.Fatalf("started a child while parent was running: %+v", state)
	}
}

func TestSupervisorCapturesShutdownCrashBeforeRunningTask(t *testing.T) {
	directory := setupSupervisorTest(t)
	t.Setenv(supervisorTestModeEnv, "crash-with-task")
	t.Setenv(supervisorTestDirEnv, directory)
	m := GetManager()
	handlerSawCrash := false
	Register(shutdownTestTask, func(context.Context, json.RawMessage) error {
		incident, ok := m.LatestCrashIncident()
		if !ok || incident.ExitCode != 7 {
			return errors.New("shutdown crash was not captured before the task")
		}
		if _, err := os.Stat(incident.ReportPath); err != nil {
			return err
		}
		handlerSawCrash = true
		return os.WriteFile(filepath.Join(directory, "task-ran"), []byte("restored"), 0644)
	})
	if code := m.RunSupervisor(context.Background(), []string{os.Args[0], ArgSupervisor}); code != 0 {
		t.Fatalf("supervisor returned code %d", code)
	}
	if !handlerSawCrash {
		t.Fatal("task did not observe the shutdown crash report")
	}
	data, err := os.ReadFile(filepath.Join(directory, "child-result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result TaskResult
	if err := json.Unmarshal(data, &result); err != nil || !result.OK || result.Type != shutdownTestTask {
		t.Fatalf("restarted child's task result = %+v, err=%v", result, err)
	}
	if _, ok := m.takeTaskResult(); ok {
		t.Fatal("restarted child did not consume the task result")
	}
	if _, err := os.Stat(m.taskJournalPath()); !os.IsNotExist(err) {
		t.Fatalf("completed task journal still exists: %v", err)
	}
}
