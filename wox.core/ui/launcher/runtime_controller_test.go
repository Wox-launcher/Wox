package launcher

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"wox/ui/contract"
)

// runtimeBlockingSettingsService blocks one selected operation after notifying the test.
type runtimeBlockingSettingsService struct {
	entered   chan<- struct{}
	release   <-chan struct{}
	operation string
	statuses  []contract.RuntimeStatus
}

func (b *runtimeBlockingSettingsService) RuntimeStatuses(_ context.Context, _ string) ([]contract.RuntimeStatus, error) {
	if b.operation == "status" {
		close(b.entered)
		<-b.release
	}
	return append([]contract.RuntimeStatus(nil), b.statuses...), nil
}

func (b *runtimeBlockingSettingsService) RestartRuntime(_ context.Context, _ string, _ string) error {
	if b.operation == "restart" {
		close(b.entered)
		<-b.release
	}
	return nil
}

type fakeRuntimeSettingsService struct {
	statuses   []contract.RuntimeStatus
	statusErr  error
	restartErr error
}

func runtimeSnapshotOnUI(ui *testUIRunner, c *runtimeSettingsController) runtimeSettingsSnapshot {
	var snapshot runtimeSettingsSnapshot
	ui.Do(func() {
		snapshot = c.Snapshot()
	})
	return snapshot
}

func (f *fakeRuntimeSettingsService) RuntimeStatuses(_ context.Context, _ string) ([]contract.RuntimeStatus, error) {
	return append([]contract.RuntimeStatus(nil), f.statuses...), f.statusErr
}

func (f *fakeRuntimeSettingsService) RestartRuntime(_ context.Context, _ string, _ string) error {
	return f.restartErr
}

func TestRuntimeControllerReloadSuccess(t *testing.T) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	c := newRuntimeSettingsController(deps)
	pluginNames := []string{"Plugin A"}
	service := &fakeRuntimeSettingsService{statuses: []contract.RuntimeStatus{
		{Runtime: "NODEJS", CanRestart: true, LoadedPluginNames: pluginNames},
		{Runtime: "PYTHON", CanRestart: false},
	}}
	c.Reload(context.Background(), service, "session")
	pluginNames[0] = "Changed"
	snap := c.Snapshot()
	if len(snap.Statuses) != 2 {
		t.Fatalf("Statuses len = %d, want 2", len(snap.Statuses))
	}
	if !snap.Loaded {
		t.Fatalf("Loaded should be true after successful reload")
	}
	if snap.Error != "" {
		t.Fatalf("Error should be empty, got %q", snap.Error)
	}
	if snap.Loading {
		t.Fatalf("Loading should be false after reload completes")
	}
	if len(snap.Statuses[0].LoadedPluginNames) != 1 || snap.Statuses[0].LoadedPluginNames[0] != "Plugin A" {
		t.Fatalf("LoadedPluginNames should be isolated from service-owned slices: %+v", snap.Statuses[0].LoadedPluginNames)
	}
	if invalidateCalled < 2 {
		t.Fatalf("Invalidate should be called at least twice, got %d", invalidateCalled)
	}
}

func TestRuntimeControllerReloadError(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newRuntimeSettingsController(deps)
	service := &fakeRuntimeSettingsService{statusErr: errors.New("network down")}
	c.Reload(context.Background(), service, "session")
	snap := c.Snapshot()
	if snap.Error == "" {
		t.Fatalf("Error should be recorded, got empty")
	}
	if snap.Loaded {
		t.Fatalf("Loaded should be false after error")
	}
	if snap.Loading {
		t.Fatalf("Loading should be false after error")
	}
}

func TestRuntimeControllerRestartSetsRestartingThenClears(t *testing.T) {
	ui := &testUIRunner{}
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }, RunOnUI: ui.Run}
	c := newRuntimeSettingsController(deps)

	// Seed statuses with a restartable Node.js host.
	enteredPost := make(chan struct{})
	releasePost := make(chan struct{})
	blockingService := &runtimeBlockingSettingsService{
		entered:   enteredPost,
		release:   releasePost,
		operation: "restart",
		statuses:  []contract.RuntimeStatus{{Runtime: "NODEJS", CanRestart: true}},
	}
	c.Reload(context.Background(), blockingService, "session")

	reloadCalled := make(chan struct{})
	var reloadOnce sync.Once
	reloadAfter := func() {
		reloadOnce.Do(func() { close(reloadCalled) })
	}

	c.Restart(context.Background(), blockingService, "session", "nodejs", reloadAfter)

	// Restarting must be set immediately after Restart returns.
	if got := runtimeSnapshotOnUI(ui, c).Restarting; got != "NODEJS" {
		t.Fatalf("Restarting = %q, want NODEJS", got)
	}

	// Wait for the goroutine to enter Post; then release it so it can clear restarting
	// and call reloadAfter.
	select {
	case <-enteredPost:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for restart Post to be entered")
	}
	close(releasePost)

	select {
	case <-reloadCalled:
	case <-time.After(2 * time.Second):
		t.Fatalf("reloadAfter was not invoked after restart completed")
	}

	// Give the goroutine a moment to finish clearing restarting after reloadAfter.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if runtimeSnapshotOnUI(ui, c).Restarting == "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := runtimeSnapshotOnUI(ui, c).Restarting; got != "" {
		t.Fatalf("Restarting = %q after goroutine completed, want empty", got)
	}
}

func TestRuntimeControllerRestartRejectsWhenAlreadyRestarting(t *testing.T) {
	ui := &testUIRunner{}
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }, RunOnUI: ui.Run}
	c := newRuntimeSettingsController(deps)

	enteredPost := make(chan struct{})
	releasePost := make(chan struct{})
	blockingService := &runtimeBlockingSettingsService{
		entered:   enteredPost,
		release:   releasePost,
		operation: "restart",
		statuses:  []contract.RuntimeStatus{{Runtime: "NODEJS", CanRestart: true}},
	}
	c.Reload(context.Background(), blockingService, "session")

	c.Restart(context.Background(), blockingService, "session", "nodejs", nil)
	if got := runtimeSnapshotOnUI(ui, c).Restarting; got != "NODEJS" {
		t.Fatalf("first Restart should set Restarting = NODEJS, got %q", got)
	}

	// Wait until the first restart's Post is in flight so the second call races
	// against a controller that is actively restarting.
	select {
	case <-enteredPost:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for first restart Post to be entered")
	}

	// Track reloadAfter invocations: the second Restart must not call reloadAfter
	// because it should no-op.
	secondReload := make(chan struct{}, 1)
	secondReloadAfter := func() {
		select {
		case secondReload <- struct{}{}:
		default:
		}
	}
	c.Restart(context.Background(), blockingService, "session", "nodejs", secondReloadAfter)

	// Second Restart should leave Restarting unchanged (still NODEJS from first call)
	// and not invoke secondReloadAfter. Give it a brief moment in case of scheduling.
	select {
	case <-secondReload:
		t.Fatalf("second Restart while already restarting must not call reloadAfter")
	case <-time.After(50 * time.Millisecond):
	}

	// Release the first restart so the goroutine cleans up.
	close(releasePost)
	if got := runtimeSnapshotOnUI(ui, c).Restarting; got != "" {
		// Spin briefly because the goroutine may need a moment to clear.
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) && runtimeSnapshotOnUI(ui, c).Restarting != "" {
			time.Sleep(5 * time.Millisecond)
		}
		if got = runtimeSnapshotOnUI(ui, c).Restarting; got != "" {
			t.Fatalf("Restarting = %q after release, want empty", got)
		}
	}
}
