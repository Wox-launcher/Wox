package diagnostic

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
	"wox/util"
	"wox/util/shell"
)

func (m *Manager) StartSupervisorDetached(ctx context.Context, waitParent bool) error {
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
	cmd := shell.BuildCommand(executable, nil, args...)
	cmd.Stdout = util.GetLogger().GetWriter()
	cmd.Stderr = util.GetLogger().GetWriter()
	cmd.Dir = shellWorkingDirectory(executable)
	// New feature: bug aware needs an external parent process. Starting the
	// supervisor before the current process exits lets it wait for a clean handoff
	// and then launch the monitored child.
	if err := cmd.Start(); err != nil {
		return err
	}
	m.AppendBreadcrumb(ctx, "supervisor_started", map[string]any{"pid": cmd.Process.Pid, "waitParent": waitParent})
	return nil
}

func (m *Manager) RunSupervisor(ctx context.Context, args []string) int {
	_ = m.EnsureDirectories()
	logFile, err := os.OpenFile(m.SupervisorLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 1
	}
	defer logFile.Close()

	waitParentPid := parseWaitParentPid(args)
	if waitParentPid > 0 {
		m.waitForParentExit(logFile, waitParentPid)
	}

	executable, err := os.Executable()
	if err != nil {
		_, _ = fmt.Fprintf(logFile, "[%s] failed to resolve executable: %v\n", time.Now().Format(time.RFC3339), err)
		return 1
	}

	childArgs := []string{ArgChild}
	cmd := exec.Command(executable, childArgs...)
	cmd.Env = os.Environ()
	cmd.Dir = shellWorkingDirectory(executable)
	cmd.Stdout = io.MultiWriter(logFile)
	cmd.Stderr = io.MultiWriter(logFile)

	startedAt := time.Now()
	_, _ = fmt.Fprintf(logFile, "[%s] starting child: %s %v\n", startedAt.Format(time.RFC3339), executable, childArgs)
	if err := cmd.Start(); err != nil {
		_, _ = fmt.Fprintf(logFile, "[%s] failed to start child: %v\n", time.Now().Format(time.RFC3339), err)
		return 1
	}
	m.AppendBreadcrumb(ctx, "supervisor_child_started", map[string]any{"pid": cmd.Process.Pid})

	waitErr := cmd.Wait()
	durationMs := time.Since(startedAt).Milliseconds()
	_, _ = fmt.Fprintf(logFile, "[%s] child exited: pid=%d durationMs=%d err=%v\n", time.Now().Format(time.RFC3339), cmd.Process.Pid, durationMs, waitErr)
	// Feature update: disabling bug aware mode should take effect immediately
	// without forcing another restart. The already-running supervisor may still
	// be waiting on this child, so it must re-check persisted state before
	// recording or exporting anything for the disabled session.
	if !m.IsEnabled() {
		_, _ = fmt.Fprintf(logFile, "[%s] bug aware disabled before child exit; supervisor will exit without exporting diagnostics\n", time.Now().Format(time.RFC3339))
		return 0
	}
	m.RecordSupervisorExit(ctx, cmd.Process.Pid, waitErr, durationMs)
	if waitErr != nil {
		if exportPath, exportErr := m.Export(ctx); exportErr == nil {
			_, _ = fmt.Fprintf(logFile, "[%s] diagnostics exported: %s\n", time.Now().Format(time.RFC3339), exportPath)
		} else {
			_, _ = fmt.Fprintf(logFile, "[%s] diagnostics export failed: %v\n", time.Now().Format(time.RFC3339), exportErr)
		}
		return 1
	}
	return 0
}

func (m *Manager) waitForParentExit(logFile io.Writer, parentPid int) {
	// The supervisor is launched while Wox is still shutting down. Waiting a
	// short bounded period prevents the child from hitting the single-instance
	// forwarding path against the process that asked to restart.
	waitStartedAt := time.Now()
	_, _ = fmt.Fprintf(logFile, "[%s] waiting for parent exit: pid=%d\n", waitStartedAt.Format(time.RFC3339), parentPid)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !isProcessRunning(parentPid) {
			_, _ = fmt.Fprintf(logFile, "[%s] parent exited: pid=%d durationMs=%d\n", time.Now().Format(time.RFC3339), parentPid, time.Since(waitStartedAt).Milliseconds())
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	// Bug diagnostics: if this timeout is reached, the restart delay is before
	// the monitored child starts. Keeping it in supervisor.log makes Windows
	// handoff delays visible without needing to infer them from breadcrumbs.
	_, _ = fmt.Fprintf(logFile, "[%s] parent wait timed out: pid=%d durationMs=%d\n", time.Now().Format(time.RFC3339), parentPid, time.Since(waitStartedAt).Milliseconds())
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

func shellWorkingDirectory(executable string) string {
	if executable == "" {
		return ""
	}
	if info, err := os.Stat(executable); err == nil && !info.IsDir() {
		return filepath.Dir(executable)
	}
	return ""
}
