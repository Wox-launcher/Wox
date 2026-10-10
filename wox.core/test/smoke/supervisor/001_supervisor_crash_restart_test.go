//go:build wox_ui_smoke

package supervisor

import (
	"context"
	"os"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
)

// Test001SupervisorCrashRestart verifies the supervisor starts Wox again after the supervised process dies.
// Flow: read the running process -> terminate that process -> wait until a new process answers.
// Evidence: supervisor.log records the crash restart, and the new process has a different pid and a visible launcher.
func Test001SupervisorCrashRestart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client := smoke.SharedClient(t, ctx)
	if err := client.Reset(ctx); err != nil {
		t.Fatalf("reset Wox before crash restart: %v", err)
	}
	previousPID := smoke.LatestStartupPID(t)
	infoFile := os.Getenv(automationdriver.SharedInfoFileEnvironment)
	if infoFile == "" {
		t.Fatal("automation info file is not configured")
	}
	info, err := automationdriver.ReadInfo(ctx, infoFile)
	if err != nil {
		t.Fatalf("read automation endpoint before crash: %v", err)
	}

	process, err := os.FindProcess(previousPID)
	if err != nil {
		t.Fatalf("find supervised Wox pid %d: %v", previousPID, err)
	}
	if err := process.Kill(); err != nil {
		t.Fatalf("terminate supervised Wox pid %d: %v", previousPID, err)
	}

	restarted, restartedPID := smoke.WaitForSupervisorReplacement(t, ctx, info, previousPID, "restarting Wox after crash")
	smoke.ShowLauncher(t, ctx, restarted)
	if restartedPID == previousPID {
		t.Fatalf("restarted pid = %d, want a new process", restartedPID)
	}
}
