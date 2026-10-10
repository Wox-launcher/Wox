package supervisor

import (
	"context"
	"fmt"
	"time"
)

const (
	crashLoopWindow             = 30 * time.Second
	crashRestartDelay           = 500 * time.Millisecond
	maxConsecutiveCrashRestarts = 2
)

var preserveRestartArgs []string

// PreserveArgOnRestart keeps a launch flag across crash restarts without
// replaying deeplinks or opened files. Callers register Wox-specific flags.
func PreserveArgOnRestart(arg string) {
	if arg == "" {
		return
	}
	for _, existing := range preserveRestartArgs {
		if existing == arg {
			return
		}
	}
	preserveRestartArgs = append(preserveRestartArgs, arg)
}

// supervisedChildArgs preserves registered flags after a crash without replaying file opens or deeplinks.
func supervisedChildArgs(args []string, firstLaunch bool) []string {
	childArgs := []string{ArgChild}
	if firstLaunch {
		return append(childArgs, forwardedProcessArgs(args)...)
	}
	for _, arg := range preserveRestartArgs {
		if hasArg(args, arg) {
			childArgs = append(childArgs, arg)
		}
	}
	return childArgs
}

// recordChildExit preserves crash evidence even when an accepted task will replace user data next.
func (m *Manager) recordChildExit(ctx context.Context, pid int, waitErr error, startedAt time.Time) time.Duration {
	duration := time.Since(startedAt)
	durationMs := duration.Milliseconds()
	m.logf("child exited: pid=%d durationMs=%d err=%v", pid, durationMs, waitErr)
	m.RecordSupervisorExit(ctx, pid, waitErr, durationMs)
	if waitErr != nil {
		m.captureCrash(ctx, pid, waitErr, startedAt, durationMs)
	}
	return duration
}

// captureCrash persists diagnostics before any replacement Wox process starts.
func (m *Manager) captureCrash(ctx context.Context, pid int, waitErr error, startedAt time.Time, durationMs int64) {
	dumpPath := m.waitForCrashArtifacts(pid, startedAt)
	exportPath, exportErr := m.ExportCrash(ctx)
	if exportErr != nil {
		m.logf("crash report export failed: %s", exportErr.Error())
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
		DumpPath:   dumpPath,
		Version:    currentVersion(),
	}
	if saveErr := m.SaveCrashIncident(incident); saveErr != nil {
		m.logf("failed to persist crash incident: %s", saveErr.Error())
	}
	if dumpPath != "" {
		m.logf("crash dump captured: %s", dumpPath)
	}
	m.logf("crash report exported: %s", exportPath)
}

// nextCrashRestart resets the crash budget after a stable run and bounds startup crash loops.
func nextCrashRestart(duration time.Duration, consecutiveCrashes int) (int, bool) {
	if duration >= crashLoopWindow {
		consecutiveCrashes = 0
	}
	consecutiveCrashes++
	return consecutiveCrashes, consecutiveCrashes <= maxConsecutiveCrashRestarts
}
