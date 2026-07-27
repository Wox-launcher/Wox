package launcher

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// runtimeBlockingBackendClient closes entered inside Post before blocking on release,
// so callers can prove the controller has already locked and entered Post.
// pathSel lets the test pick which path to block on ("/runtime/status" or "/runtime/restart").
type runtimeBlockingBackendClient struct {
	entered  chan<- struct{}
	release  <-chan struct{}
	pathSel  string
	statuses []runtimeStatus
}

func (b *runtimeBlockingBackendClient) Post(_ context.Context, path string, _ any, out any) error {
	if path == b.pathSel {
		close(b.entered)
		<-b.release
	}
	if ptr, ok := out.(*[]runtimeStatus); ok {
		*ptr = append([]runtimeStatus(nil), b.statuses...)
	}
	return nil
}

func TestRuntimeControllerReloadSuccess(t *testing.T) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	c := newRuntimeSettingsController(deps)
	client := &fakeBackendClient{runtimeStatuses: []runtimeStatus{
		{Runtime: "NODEJS", CanRestart: true},
		{Runtime: "PYTHON", CanRestart: false},
	}}
	c.Reload(context.Background(), client)
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
	if invalidateCalled < 2 {
		t.Fatalf("Invalidate should be called at least twice, got %d", invalidateCalled)
	}
}

func TestRuntimeControllerReloadError(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newRuntimeSettingsController(deps)
	client := &fakeBackendClient{err: errors.New("network down")}
	c.Reload(context.Background(), client)
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
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newRuntimeSettingsController(deps)

	// Seed statuses with a restartable Node.js host.
	enteredPost := make(chan struct{})
	releasePost := make(chan struct{})
	blockingClient := &runtimeBlockingBackendClient{
		entered:  enteredPost,
		release:  releasePost,
		pathSel:  "/runtime/restart",
		statuses: []runtimeStatus{{Runtime: "NODEJS", CanRestart: true}},
	}
	c.Reload(context.Background(), blockingClient)

	reloadCalled := make(chan struct{})
	var reloadOnce sync.Once
	reloadAfter := func() {
		reloadOnce.Do(func() { close(reloadCalled) })
	}

	c.Restart(context.Background(), blockingClient, "nodejs", reloadAfter)

	// Restarting must be set immediately after Restart returns.
	if got := c.Snapshot().Restarting; got != "NODEJS" {
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
		if c.Snapshot().Restarting == "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if got := c.Snapshot().Restarting; got != "" {
		t.Fatalf("Restarting = %q after goroutine completed, want empty", got)
	}
}

func TestRuntimeControllerRestartRejectsWhenAlreadyRestarting(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newRuntimeSettingsController(deps)

	enteredPost := make(chan struct{})
	releasePost := make(chan struct{})
	blockingClient := &runtimeBlockingBackendClient{
		entered:  enteredPost,
		release:  releasePost,
		pathSel:  "/runtime/restart",
		statuses: []runtimeStatus{{Runtime: "NODEJS", CanRestart: true}},
	}
	c.Reload(context.Background(), blockingClient)

	c.Restart(context.Background(), blockingClient, "nodejs", nil)
	if got := c.Snapshot().Restarting; got != "NODEJS" {
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
	c.Restart(context.Background(), blockingClient, "nodejs", secondReloadAfter)

	// Second Restart should leave Restarting unchanged (still NODEJS from first call)
	// and not invoke secondReloadAfter. Give it a brief moment in case of scheduling.
	select {
	case <-secondReload:
		t.Fatalf("second Restart while already restarting must not call reloadAfter")
	case <-time.After(50 * time.Millisecond):
	}

	// Release the first restart so the goroutine cleans up.
	close(releasePost)
	if got := c.Snapshot().Restarting; got != "" {
		// Spin briefly because the goroutine may need a moment to clear.
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) && c.Snapshot().Restarting != "" {
			time.Sleep(5 * time.Millisecond)
		}
		if got = c.Snapshot().Restarting; got != "" {
			t.Fatalf("Restarting = %q after release, want empty", got)
		}
	}
}
