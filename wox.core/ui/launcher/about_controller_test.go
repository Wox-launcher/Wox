package launcher

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type fakeBackendClient struct {
	mu              sync.Mutex
	version         string
	stats           usageStatsData
	runtimeStatuses []runtimeStatus
	channelVersions []updateChannelVersion
	err             error
}

func (f *fakeBackendClient) Post(_ context.Context, _ string, _ any, out any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	if ptr, ok := out.(*string); ok {
		*ptr = f.version
	}
	if ptr, ok := out.(*usageStatsData); ok {
		*ptr = f.stats
	}
	if ptr, ok := out.(*[]runtimeStatus); ok {
		*ptr = append([]runtimeStatus(nil), f.runtimeStatuses...)
	}
	if ptr, ok := out.(*[]updateChannelVersion); ok {
		*ptr = append([]updateChannelVersion(nil), f.channelVersions...)
	}
	return nil
}

func TestAboutControllerReloadSuccess(t *testing.T) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	c := newAboutSettingsController(deps)
	client := &fakeBackendClient{version: "1.2.3"}
	c.Reload(context.Background(), client)
	snap := c.Snapshot()
	if snap.Version != "1.2.3" || !snap.Loaded || snap.Error != "" {
		t.Fatalf("after reload: %+v", snap)
	}
	if invalidateCalled < 2 {
		t.Fatalf("Invalidate should be called at least twice, got %d", invalidateCalled)
	}
}

func TestAboutControllerReloadError(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newAboutSettingsController(deps)
	client := &fakeBackendClient{err: errors.New("network down")}
	c.Reload(context.Background(), client)
	snap := c.Snapshot()
	if snap.Error == "" || snap.Loaded {
		t.Fatalf("error should be recorded: %+v", snap)
	}
}

func TestAboutControllerReloadConcurrent(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newAboutSettingsController(deps)
	client := &fakeBackendClient{version: "v"}
	// First call claims loading; second should no-op.
	go c.Reload(context.Background(), client)
	c.Reload(context.Background(), client)
	// No deadlock/panic; version eventually set.
}
