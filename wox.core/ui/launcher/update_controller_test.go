package launcher

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestUpdateControllerReloadSuccess(t *testing.T) {
	invalidateCalled := 0
	deps := CommonDeps{
		Invalidate: func() { invalidateCalled++ },
		Translate:  func(s string) string { return s },
	}
	c := newUpdateSettingsController(deps)
	client := &fakeBackendClient{channelVersions: []updateChannelVersion{
		{Channel: "stable", LatestVersion: "1.2.3"},
		{Channel: "beta", LatestVersion: "1.3.0"},
	}}
	trailersArg := []updateChannelVersion(nil)
	called := 0
	applyTrailers := func(versions []updateChannelVersion) {
		called++
		trailersArg = versions
	}
	c.Reload(context.Background(), client, applyTrailers)
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
	deps := CommonDeps{Invalidate: func() {}, Translate: func(s string) string { return s }}
	c := newUpdateSettingsController(deps)

	// Handshake channels: enteredPost is closed once the first Reload is inside Post
	// (loading flag already set to true), and releaseFirst unblocks it so its Post can return.
	enteredPost := make(chan struct{})
	releaseFirst := make(chan struct{})
	var firstCallReturned sync.WaitGroup
	firstCallReturned.Add(1)

	blockingClient := &updateBlockingBackendClient{
		entered: enteredPost,
		release: releaseFirst,
		response: []updateChannelVersion{
			{Channel: "stable", LatestVersion: "1.2.3"},
		},
	}
	go func() {
		defer firstCallReturned.Done()
		c.Reload(context.Background(), blockingClient, nil)
	}()

	<-enteredPost
	// Second reload runs while the first is still inside Post; the loading guard must no-op it.
	secondClient := &fakeBackendClient{channelVersions: []updateChannelVersion{
		{Channel: "beta", LatestVersion: "9.9.9"},
	}}
	called := 0
	c.Reload(context.Background(), secondClient, func([]updateChannelVersion) { called++ })

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
	client := &fakeBackendClient{channelVersions: []updateChannelVersion{
		{Channel: "stable", LatestVersion: "1.2.3"},
	}}
	c.Reload(context.Background(), client, nil)

	// Second reload must no-op because versions are already loaded (len > 0 guard).
	secondClient := &fakeBackendClient{channelVersions: []updateChannelVersion{
		{Channel: "beta", LatestVersion: "9.9.9"},
	}}
	called := 0
	c.Reload(context.Background(), secondClient, func([]updateChannelVersion) { called++ })

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
	client := &fakeBackendClient{err: errors.New("network down")}
	called := 0
	c.Reload(context.Background(), client, func([]updateChannelVersion) { called++ })
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

// updateBlockingBackendClient closes entered inside Post before blocking on release,
// so callers can prove the controller has already set channelsLoading=true and entered Post.
type updateBlockingBackendClient struct {
	entered  chan<- struct{}
	release  <-chan struct{}
	response []updateChannelVersion
}

func (b *updateBlockingBackendClient) Post(_ context.Context, _ string, _ any, out any) error {
	close(b.entered)
	<-b.release
	if ptr, ok := out.(*[]updateChannelVersion); ok {
		*ptr = append([]updateChannelVersion(nil), b.response...)
	}
	return nil
}
