package supervisor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const parentWaitTimeout = 10 * time.Second
const parentWaitWithTask = 30 * time.Second

// logStartup records the supervisor process itself, separate from the Wox child log.
func (m *Manager) logStartup(args []string) {
	executable, err := os.Executable()
	if err != nil {
		executable = "<error>"
	}
	m.logf("------------------------------")
	m.logf("supervisor starting: %s", currentVersion())
	m.logf("golang version: %s", strings.TrimPrefix(runtime.Version(), "go"))
	m.logf("data location: %s", dataDirectory())
	m.logf("startup pid: %d, executable: %s, args: %v", os.Getpid(), executable, args)
}

// StartSupervisorDetached launches a supervisor. Explicit child arguments replace the current launch arguments.
func (m *Manager) StartSupervisorDetached(ctx context.Context, waitParent bool, childArgs ...string) error {
	if err := m.EnsureDirectories(); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{ArgSupervisor}
	if waitParent {
		args = append(args, ArgWaitParent, strconv.Itoa(os.Getpid()))
	}
	if len(childArgs) == 0 {
		childArgs = forwardedProcessArgs(os.Args)
	}
	args = append(args, childArgs...)
	cmd := exec.Command(executable, args...)
	address := newControlAddress()
	// The new supervisor and all its Wox children inherit their own endpoint;
	// an older supervisor may still be serving while this process shuts down.
	cmd.Env = append(os.Environ(), controlAddressEnv+"="+address)
	cmd.Dir = executableDirectory(executable)
	hideWindow(cmd)
	logFile, openErr := os.OpenFile(m.SupervisorLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if openErr == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	if err := cmd.Start(); err != nil {
		if logFile != nil {
			_ = logFile.Close()
		}
		return err
	}
	if logFile != nil {
		_ = logFile.Close()
	}
	SetControlAddress(address)
	m.AppendBreadcrumb(ctx, "supervisor_started", map[string]any{"pid": cmd.Process.Pid, "waitParent": waitParent})
	return nil
}

// RunSupervisor watches Wox until it exits cleanly, restarting it after a crash
// and running any task that was accepted for the gap between processes.
func (m *Manager) RunSupervisor(ctx context.Context, args []string) int {
	if err := m.EnsureDirectories(); err != nil {
		return 1
	}
	logFile, err := os.OpenFile(m.SupervisorLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 1
	}
	defer logFile.Close()
	m.logStartup(args)

	m.loadTaskJournal()
	listener, err := listenControl()
	if err != nil {
		m.logf("control listen: %s", err.Error())
		return 1
	}
	// Close synchronously before main calls os.Exit, including removal of our Unix socket.
	defer listener.Close()
	controlCtx, stopControl := context.WithCancel(ctx)
	defer stopControl()
	go m.serveControl(controlCtx, listener)

	waitParentPid := parseWaitParentPid(args)
	if waitParentPid > 0 {
		timeout := parentWaitTimeout
		if m.hasPendingTask() {
			timeout = parentWaitWithTask
		}
		if err := m.waitForParentExit(ctx, waitParentPid, timeout); err != nil {
			m.logf("parent exit not confirmed; leaving accepted task queued: %s", err.Error())
			return 1
		}
	}

	executable, err := os.Executable()
	if err != nil {
		m.logf("failed to resolve executable: %s", err.Error())
		return 1
	}

	consecutiveCrashes := 0
	for firstLaunch := true; ; firstLaunch = false {
		ranTask := m.runPendingTask(ctx)
		childArgs := supervisedChildArgs(args, firstLaunch && !ranTask)
		cmd, startedAt, startErr := m.startSupervisedChild(ctx, logFile, executable, childArgs)
		if startErr != nil {
			return 1
		}

		waitErr := cmd.Wait()
		duration := m.recordChildExit(ctx, cmd.Process.Pid, waitErr, startedAt)
		if m.hasPendingTask() {
			m.logf("child exited with a pending task")
			continue
		}
		if waitErr == nil {
			return 0
		}

		consecutiveCrashes, restart := nextCrashRestart(duration, consecutiveCrashes)
		if !restart {
			m.logf("crash restart limit reached; supervisor will stop")
			m.AppendBreadcrumb(ctx, "crash_restart_limit_reached", map[string]any{"consecutiveCrashes": consecutiveCrashes})
			return 1
		}
		m.logf("restarting Wox after crash: attempt=%d delayMs=%d", consecutiveCrashes, crashRestartDelay.Milliseconds())
		m.AppendBreadcrumb(ctx, "crash_restart_scheduled", map[string]any{"attempt": consecutiveCrashes, "delayMs": crashRestartDelay.Milliseconds()})
		time.Sleep(crashRestartDelay)
	}
}

func (m *Manager) startSupervisedChild(ctx context.Context, logFile io.Writer, executable string, childArgs []string) (*exec.Cmd, time.Time, error) {
	cmd := exec.Command(executable, childArgs...)
	cmd.Env = os.Environ()
	cmd.Dir = executableDirectory(executable)
	cmd.Stdout = io.MultiWriter(logFile)
	cmd.Stderr = io.MultiWriter(logFile)
	hideWindow(cmd)
	startedAt := time.Now()
	m.logf("starting child: %s %v", executable, childArgs)
	if err := cmd.Start(); err != nil {
		m.logf("failed to start child: %s", err.Error())
		return nil, startedAt, err
	}
	m.AppendBreadcrumb(ctx, "supervisor_child_started", map[string]any{"pid": cmd.Process.Pid})
	return cmd, startedAt, nil
}

// waitForParentExit confirms that the launching Wox released its files before any task or child starts.
func (m *Manager) waitForParentExit(ctx context.Context, parentPid int, timeout time.Duration) error {
	waitStartedAt := time.Now()
	m.logf("waiting for the Wox process that launched the supervisor to exit before starting the supervised instance: pid=%d", parentPid)
	deadline := time.Now().Add(timeout)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if m.hasPendingTask() && timeout < parentWaitWithTask {
			deadline = waitStartedAt.Add(parentWaitWithTask)
			timeout = parentWaitWithTask
		}
		if !IsProcessRunning(parentPid) {
			m.logf("launching Wox has exited, starting the supervised instance: pid=%d durationMs=%d", parentPid, time.Since(waitStartedAt).Milliseconds())
			return nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("launching Wox is still running after %s: pid=%d", timeout, parentPid)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(min(200*time.Millisecond, remaining)):
		}
	}
}

func parseWaitParentPid(args []string) int {
	for i, arg := range args {
		if arg == ArgWaitParent && i+1 < len(args) {
			pid, _ := strconv.Atoi(args[i+1])
			return pid
		}
	}
	return 0
}

func forwardedProcessArgs(args []string) []string {
	forwarded := make([]string, 0, len(args))
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case ArgSupervisor, ArgChild:
			continue
		case ArgWaitParent:
			i++
			continue
		default:
			forwarded = append(forwarded, args[i])
		}
	}
	return forwarded
}

func executableDirectory(executable string) string {
	if executable == "" {
		return ""
	}
	if info, err := os.Stat(executable); err == nil && !info.IsDir() {
		return filepath.Dir(executable)
	}
	return ""
}
