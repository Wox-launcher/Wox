package launcher

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type fakeVersionSettingsService struct {
	version string
	err     error
}

func (f *fakeVersionSettingsService) Version(_ context.Context, _ string) (string, error) {
	return f.version, f.err
}

func TestAboutControllerReloadSuccess(t *testing.T) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	c := newAboutSettingsController(deps)
	service := &fakeVersionSettingsService{version: "1.2.3"}
	c.Reload(context.Background(), service, "session")
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
	service := &fakeVersionSettingsService{err: errors.New("network down")}
	c.Reload(context.Background(), service, "session")
	snap := c.Snapshot()
	if snap.Error == "" || snap.Loaded {
		t.Fatalf("error should be recorded: %+v", snap)
	}
}

func TestAboutControllerReloadConcurrent(t *testing.T) {
	ui := &testUIRunner{}
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }, RunOnUI: ui.Run}
	c := newAboutSettingsController(deps)
	service := &fakeVersionSettingsService{version: "v"}
	// First call claims loading; second should no-op.
	var reloads sync.WaitGroup
	reloads.Add(1)
	go func() {
		defer reloads.Done()
		c.Reload(context.Background(), service, "session")
	}()
	c.Reload(context.Background(), service, "session")
	reloads.Wait()
	if snap := c.Snapshot(); snap.Version != "v" {
		t.Fatalf("Version = %q, want v", snap.Version)
	}
}
