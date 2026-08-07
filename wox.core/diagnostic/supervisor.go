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
	"wox/updater"
	"wox/util"
	"wox/util/shell"
)

const (
	crashLoopWindow             = 30 * time.Second
	crashRestartDelay           = 500 * time.Millisecond
	maxConsecutiveCrashRestarts = 2
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
	args = append(args, forwardedProcessArgs(os.Args)...)
	cmd := shell.BuildCommand(executable, nil, args...)
	cmd.Stdout = util.GetLogger().GetWriter()
	cmd.Stderr = util.GetLogger().GetWriter()
	cmd.Dir = shellWorkingDirectory(executable)
	// Starting the supervisor before the bootstrap process exits lets it wait for
	// a clean handoff and then launch the monitored application process.
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

	initialChildArgs := append([]string{ArgChild}, forwardedProcessArgs(args)...)
	consecutiveCrashes := 0
	for firstLaunch := true; ; firstLaunch = false {
		childArgs := []string{ArgChild}
		if firstLaunch {
			childArgs = initialChildArgs
		}
		cmd, startedAt, startErr := m.startSupervisedChild(ctx, logFile, executable, childArgs)
		if startErr != nil {
			return 1
		}

		waitErr := cmd.Wait()
		duration := time.Since(startedAt)
		durationMs := duration.Milliseconds()
		_, _ = fmt.Fprintf(logFile, "[%s] child exited: pid=%d durationMs=%d err=%v\n", time.Now().Format(time.RFC3339), cmd.Process.Pid, durationMs, waitErr)
		m.RecordSupervisorExit(ctx, cmd.Process.Pid, waitErr, durationMs)
		if waitErr == nil {
			return 0
		}

		m.captureCrash(ctx, logFile, cmd.Process.Pid, waitErr, startedAt, durationMs)
		consecutiveCrashes, restart := nextCrashRestart(duration, consecutiveCrashes)
		if !restart {
			_, _ = fmt.Fprintf(logFile, "[%s] crash restart limit reached; supervisor will stop\n", time.Now().Format(time.RFC3339))
			m.AppendBreadcrumb(ctx, "crash_restart_limit_reached", map[string]any{"consecutiveCrashes": consecutiveCrashes})
			return 1
		}
		_, _ = fmt.Fprintf(logFile, "[%s] restarting Wox after crash: attempt=%d delayMs=%d\n", time.Now().Format(time.RFC3339), consecutiveCrashes, crashRestartDelay.Milliseconds())
		m.AppendBreadcrumb(ctx, "crash_restart_scheduled", map[string]any{"attempt": consecutiveCrashes, "delayMs": crashRestartDelay.Milliseconds()})
		time.Sleep(crashRestartDelay)
	}
}

// startSupervisedChild starts one monitored Wox process and connects its output to the supervisor log.
func (m *Manager) startSupervisedChild(ctx context.Context, logFile io.Writer, executable string, childArgs []string) (*exec.Cmd, time.Time, error) {
	cmd := exec.Command(executable, childArgs...)
	cmd.Env = os.Environ()
	cmd.Dir = shellWorkingDirectory(executable)
	cmd.Stdout = io.MultiWriter(logFile)
	cmd.Stderr = io.MultiWriter(logFile)
	startedAt := time.Now()
	_, _ = fmt.Fprintf(logFile, "[%s] starting child: %s %v\n", startedAt.Format(time.RFC3339), executable, childArgs)
	if err := cmd.Start(); err != nil {
		_, _ = fmt.Fprintf(logFile, "[%s] failed to start child: %v\n", time.Now().Format(time.RFC3339), err)
		return nil, startedAt, err
	}
	m.AppendBreadcrumb(ctx, "supervisor_child_started", map[string]any{"pid": cmd.Process.Pid})
	return cmd, startedAt, nil
}

// captureCrash persists diagnostics before any replacement Wox process starts.
func (m *Manager) captureCrash(ctx context.Context, logFile io.Writer, pid int, waitErr error, startedAt time.Time, durationMs int64) {
	m.waitForCrashArtifacts(startedAt)
	exportPath, exportErr := m.ExportCrash(ctx)
	if exportErr != nil {
		_, _ = fmt.Fprintf(logFile, "[%s] crash report export failed: %v\n", time.Now().Format(time.RFC3339), exportErr)
		return
	}
	exitCode, signalName := ResolveProcessExit(waitErr)
	detectedAt := time.Now().UnixMilli()
	incident := CrashIncident{
		ID:         fmt.Sprintf("%d-%d", detectedAt, pid),
		DetectedAt: detectedAt,
		PID:        pid,
		ExitCode:   exitCode,
		Signal:     signalName,
		DurationMs: durationMs,
		ReportPath: exportPath,
		Version:    updater.CURRENT_VERSION,
	}
	if saveErr := m.SaveCrashIncident(incident); saveErr != nil {
		_, _ = fmt.Fprintf(logFile, "[%s] failed to persist crash incident: %v\n", time.Now().Format(time.RFC3339), saveErr)
	}
	_, _ = fmt.Fprintf(logFile, "[%s] crash report exported: %s\n", time.Now().Format(time.RFC3339), exportPath)
}

// nextCrashRestart resets the crash budget after a stable run and bounds startup crash loops.
func nextCrashRestart(duration time.Duration, consecutiveCrashes int) (int, bool) {
	if duration >= crashLoopWindow {
		consecutiveCrashes = 0
	}
	consecutiveCrashes++
	return consecutiveCrashes, consecutiveCrashes <= maxConsecutiveCrashRestarts
}

func forwardedProcessArgs(args []string) []string {
	// Supervisor-only arguments must never leak into the monitored child, while
	// updater flags and protocol URLs still need to reach the real application.
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

func (m *Manager) waitForParentExit(logFile io.Writer, parentPid int) {
	// The supervisor is launched while Wox is still shutting down. Waiting a
	// short bounded period prevents the child from hitting the single-instance
	// forwarding path against the process that asked to restart.
	waitStartedAt := time.Now()
	_, _ = fmt.Fprintf(logFile, "[%s] waiting for parent exit: pid=%d\n", waitStartedAt.Format(time.RFC3339), parentPid)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !IsProcessRunning(parentPid) {
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
