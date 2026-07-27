package launcher

import (
	"context"
	"errors"
	"sync"
	"testing"

	"wox/ui/contract"
)

func TestUpdateControllerReloadSuccess(t *testing.T) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	c := newUpdateSettingsController(deps)
	service := &fakeUpdateSettingsService{versions: []contract.UpdateChannelVersion{
		{Channel: "stable", LatestVersion: "1.2.3"},
		{Channel: "beta", LatestVersion: "1.3.0"},
	}}
	trailersArg := []updateChannelVersion(nil)
	called := 0
	applyTrailers := func(versions []updateChannelVersion) {
		called++
		trailersArg = versions
	}
	c.Reload(context.Background(), service, "session", applyTrailers)
	snap := c.Snapshot()
	if len(snap.ChannelVersions) != 2 {
		t.Fatalf("ChannelVersions len = %d, want 2", len(snap.ChannelVersions))
	}
	if snap.ChannelVersions[0].Channel != "stable" || snap.ChannelVersions[1].LatestVersion != "1.3.0" {
		t.Fatalf("ChannelVersions = %+v", snap.ChannelVersions)
	}
	if snap.ChannelsLoading {
		t.Fatalf("ChannelsLoading should be false after reload completes")
	}
	if called != 1 {
		t.Fatalf("applyTrailers should be called once, got %d", called)
	}
	if len(trailersArg) != 2 {
		t.Fatalf("applyTrailers received len = %d, want 2", len(trailersArg))
	}
	if invalidateCalled < 2 {
		t.Fatalf("Invalidate should be called at least twice, got %d", invalidateCalled)
	}
}

func TestUpdateControllerReloadSkipsWhenAlreadyLoading(t *testing.T) {
	ui := &testUIRunner{}
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }, RunOnUI: ui.Run}
	c := newUpdateSettingsController(deps)

	// Handshake channels: enteredPost is closed once the first Reload is inside Post
	// (loading flag already set to true), and releaseFirst unblocks it so its Post can return.
	enteredPost := make(chan struct{})
	releaseFirst := make(chan struct{})
	var firstCallReturned sync.WaitGroup
	firstCallReturned.Add(1)

	blockingService := &updateBlockingSettingsService{
		entered: enteredPost,
		release: releaseFirst,
		response: []contract.UpdateChannelVersion{
			{Channel: "stable", LatestVersion: "1.2.3"},
		},
	}
	go func() {
		defer firstCallReturned.Done()
		c.Reload(context.Background(), blockingService, "session", nil)
	}()

	<-enteredPost
	// Second reload runs while the first is still inside Post; the loading guard must no-op it.
	secondService := &fakeUpdateSettingsService{versions: []contract.UpdateChannelVersion{
		{Channel: "beta", LatestVersion: "9.9.9"},
	}}
	called := 0
	c.Reload(context.Background(), secondService, "session", func([]updateChannelVersion) { called++ })

	close(releaseFirst)
	firstCallReturned.Wait()

	snap := c.Snapshot()
	if len(snap.ChannelVersions) != 1 {
		t.Fatalf("second Reload should have been skipped, ChannelVersions len = %d, want 1", len(snap.ChannelVersions))
	}
	if snap.ChannelVersions[0].Channel != "stable" {
		t.Fatalf("ChannelVersions[0].Channel = %q, want stable", snap.ChannelVersions[0].Channel)
	}
	if called != 0 {
		t.Fatalf("applyTrailers should not be called for skipped second Reload, got %d", called)
	}
}

func TestUpdateControllerReloadSkipsWhenAlreadyLoaded(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newUpdateSettingsController(deps)
	service := &fakeUpdateSettingsService{versions: []contract.UpdateChannelVersion{
		{Channel: "stable", LatestVersion: "1.2.3"},
	}}
	c.Reload(context.Background(), service, "session", nil)

	// Second reload must no-op because versions are already loaded (len > 0 guard).
	secondService := &fakeUpdateSettingsService{versions: []contract.UpdateChannelVersion{
		{Channel: "beta", LatestVersion: "9.9.9"},
	}}
	called := 0
	c.Reload(context.Background(), secondService, "session", func([]updateChannelVersion) { called++ })

	snap := c.Snapshot()
	if len(snap.ChannelVersions) != 1 {
		t.Fatalf("second Reload should have been skipped, ChannelVersions len = %d, want 1", len(snap.ChannelVersions))
	}
	if snap.ChannelVersions[0].Channel != "stable" {
		t.Fatalf("ChannelVersions[0].Channel = %q, want stable", snap.ChannelVersions[0].Channel)
	}
	if called != 0 {
		t.Fatalf("applyTrailers should not be called for skipped second Reload, got %d", called)
	}
}

func TestUpdateControllerReloadErrorDoesNotStoreVersions(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newUpdateSettingsController(deps)
	service := &fakeUpdateSettingsService{err: errors.New("network down")}
	called := 0
	c.Reload(context.Background(), service, "session", func([]updateChannelVersion) { called++ })
	snap := c.Snapshot()
	if len(snap.ChannelVersions) != 0 {
		t.Fatalf("ChannelVersions should be empty on error, got len %d", len(snap.ChannelVersions))
	}
	if snap.ChannelsLoading {
		t.Fatalf("ChannelsLoading should be false after error completes")
	}
	if called != 0 {
		t.Fatalf("applyTrailers should not be called on error, got %d", called)
	}
}

type fakeUpdateSettingsService struct {
	versions []contract.UpdateChannelVersion
	err      error
}

func (f *fakeUpdateSettingsService) UpdateChannelVersions(_ context.Context, _ string) ([]contract.UpdateChannelVersion, error) {
	return append([]contract.UpdateChannelVersion(nil), f.versions...), f.err
}

// updateBlockingSettingsService closes entered inside UpdateChannelVersions before blocking on release.
type updateBlockingSettingsService struct {
	entered  chan<- struct{}
	release  <-chan struct{}
	response []contract.UpdateChannelVersion
}

func (b *updateBlockingSettingsService) UpdateChannelVersions(_ context.Context, _ string) ([]contract.UpdateChannelVersion, error) {
	close(b.entered)
	<-b.release
	return append([]contract.UpdateChannelVersion(nil), b.response...), nil
}
