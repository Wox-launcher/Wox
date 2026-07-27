package launcher

import (
	"context"
	"errors"
	"sync"
	"testing"

	"wox/ui/contract"
)

type fakeUsageSettingsService struct {
	stats contract.UsageStats
	err   error
}

func (f *fakeUsageSettingsService) UsageStats(_ context.Context, _ string, _ string) (contract.UsageStats, error) {
	return f.stats, f.err
}

func TestUsageControllerReloadSuccess(t *testing.T) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	c := newUsageSettingsController(deps)
	service := &fakeUsageSettingsService{stats: contract.UsageStats{PeriodOpened: 42, Period: "7d"}}
	c.Reload(context.Background(), service, "session", "7d")
	snap := c.Snapshot()
	if snap.Stats.PeriodOpened != 42 {
		t.Fatalf("Stats.PeriodOpened = %d, want 42", snap.Stats.PeriodOpened)
	}
	if snap.Period != "7d" {
		t.Fatalf("Period = %q, want 7d", snap.Period)
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

func TestUsageControllerReloadError(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newUsageSettingsController(deps)
	service := &fakeUsageSettingsService{err: errors.New("network down")}
	c.Reload(context.Background(), service, "session", "7d")
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

func TestUsageControllerReloadStaleResponseIgnored(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newUsageSettingsController(deps)

	// Handshake channels: enteredPost is closed once the first Reload is inside Post
	// (revision already bumped to 1), and releaseFirst unblocks it so its Post can return.
	enteredPost := make(chan struct{})
	releaseFirst := make(chan struct{})
	var firstCallReturned sync.WaitGroup
	firstCallReturned.Add(1)

	blockingService := &signalBlockingUsageService{
		entered:  enteredPost,
		release:  releaseFirst,
		response: contract.UsageStats{PeriodOpened: 1, Period: "7d"},
	}
	go func() {
		defer firstCallReturned.Done()
		c.Reload(context.Background(), blockingService, "session", "7d")
	}()

	<-enteredPost
	// B is guaranteed to start after A has already bumped revision to 1,
	// so B bumps to 2 and returns immediately with the fresh stats.
	freshService := &fakeUsageSettingsService{stats: contract.UsageStats{PeriodOpened: 99, Period: "30d"}}
	c.Reload(context.Background(), freshService, "session", "30d")

	// Release A's Post; A checks 1 != 2 and returns without writing.
	close(releaseFirst)
	firstCallReturned.Wait()

	snap := c.Snapshot()
	if snap.Stats.PeriodOpened != 99 {
		t.Fatalf("stale response should be ignored: Stats.PeriodOpened = %d, want 99", snap.Stats.PeriodOpened)
	}
	if snap.Period != "30d" {
		t.Fatalf("Period = %q, want 30d", snap.Period)
	}
}

func TestUsageControllerCurrentPeriodDefaults(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newUsageSettingsController(deps)
	if got := c.CurrentPeriod(); got != "30d" {
		t.Fatalf("CurrentPeriod() = %q, want 30d", got)
	}
}

func TestUsageControllerSetShareError(t *testing.T) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	c := newUsageSettingsController(deps)
	c.SetShareError("oops")
	snap := c.Snapshot()
	if snap.Error != "oops" {
		t.Fatalf("Error = %q, want oops", snap.Error)
	}
	if invalidateCalled < 1 {
		t.Fatalf("Invalidate should be called, got %d", invalidateCalled)
	}
}

// signalBlockingUsageService closes entered before blocking on release.
type signalBlockingUsageService struct {
	entered  chan<- struct{}
	release  <-chan struct{}
	response contract.UsageStats
}

func (b *signalBlockingUsageService) UsageStats(_ context.Context, _ string, _ string) (contract.UsageStats, error) {
	close(b.entered)
	<-b.release
	return b.response, nil
}
