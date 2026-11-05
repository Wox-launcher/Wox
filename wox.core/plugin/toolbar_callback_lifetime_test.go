package plugin

import (
	"context"
	"testing"
	"time"

	"wox/util"

	"github.com/stretchr/testify/require"
)

type toolbarCallbackTestPlugin struct {
	fakeLifecyclePlugin
	released chan string
	started  chan string
	finish   chan struct{}
}

func (p *toolbarCallbackTestPlugin) CreateManagedToolbarMsgActionProxy(_ string, id string) func(context.Context, ToolbarMsgActionContext) {
	return func(_ context.Context, _ ToolbarMsgActionContext) {
		p.started <- id
		<-p.finish
	}
}

func (p *toolbarCallbackTestPlugin) RetireToolbarSnapshot(_ context.Context, id string) {
	p.released <- id
}

// TestManagedToolbarCallbacksFollowMessageLifetime covers dropped, replaced, and in-flight callbacks.
func TestManagedToolbarCallbacksFollowMessageLifetime(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
	ui := &presentationTestUI{}
	manager.AttachUI(ui)
	host := &toolbarCallbackTestPlugin{released: make(chan string, 10), started: make(chan string, 1), finish: make(chan struct{})}
	instance := newInitTestInstance(host)
	instance.Metadata.Id = "owner"
	manager.lifecycle.appendPluginInstance(instance)
	ctx := util.WithSessionContext(context.Background(), "window")
	message := func(id, callbackID string) ToolbarMsg {
		return ToolbarMsg{Id: id, HideIcon: true, IsPersist: true, HostSnapshotId: callbackID, Actions: []ToolbarMsgAction{{Id: "run"}}}
	}
	manager.ShowToolbarMsg(ctx, instance, message("ignored", "ignored-token"))
	select {
	case ids := <-host.released:
		require.Equal(t, "ignored-token", ids)
	case <-time.After(time.Second):
		t.Fatal("rejected message did not release host callbacks")
	}
	manager.BeginSessionQuery("window", "query")
	manager.HandleQueryLifecycle(ctx, Query{Id: "query", SessionId: "window", Type: QueryTypeSelection}, nil)
	manager.presentation.selectResultContext(ctx, "window", "query", "result", instance.Metadata.Id)
	manager.ShowToolbarMsg(ctx, instance, message("old", "old-token"))
	manager.ShowToolbarMsg(ctx, instance, message("new", "new-token"))
	select {
	case ids := <-host.released:
		require.Equal(t, "old-token", ids)
	case <-time.After(time.Second):
		t.Fatal("replaced message did not release host callbacks")
	}
	done := make(chan error, 1)
	go func() { done <- manager.ExecuteToolbarMsgAction(ctx, "window", "new", "run") }()
	select {
	case id := <-host.started:
		require.Equal(t, "new-token", id)
	case <-time.After(time.Second):
		t.Fatal("action did not start")
	}
	manager.ClearToolbarMsg(ctx, instance, "new")
	select {
	case ids := <-host.released:
		t.Fatalf("released an executing callback: %v", ids)
	default:
	}
	close(host.finish)
	require.NoError(t, <-done)
	select {
	case ids := <-host.released:
		require.Equal(t, "new-token", ids)
	case <-time.After(time.Second):
		t.Fatal("completed action did not release its retired callback")
	}
}

// TestManagedToolbarCallbacksExpireAndDiscardQueuedMessages exercises non-visible ownership transitions.
func TestManagedToolbarCallbacksExpireAndDiscardQueuedMessages(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
	manager.AttachUI(&presentationTestUI{})
	var expiry, hold func()
	manager.presentation.scheduleToolbarExpiry = func(_ time.Duration, callback func()) { expiry = callback }
	manager.presentation.scheduleToolbarHold = func(_ time.Duration, callback func()) { hold = callback }
	host := &toolbarCallbackTestPlugin{released: make(chan string, 10)}
	a, b := newInitTestInstance(host), newInitTestInstance(host)
	a.Metadata.Id, b.Metadata.Id = "first", "second"
	manager.lifecycle.appendPluginInstance(a)
	manager.lifecycle.appendPluginInstance(b)
	ctx := util.WithSessionContext(context.Background(), "window")
	manager.BeginSessionQuery("window", "query")
	manager.HandleQueryLifecycle(ctx, Query{Id: "query", SessionId: "window", Type: QueryTypeSelection}, nil)
	manager.presentation.selectResultContext(ctx, "window", "query", "first", a.Metadata.Id)
	msg := func(id, token string) ToolbarMsg {
		return ToolbarMsg{Id: id, HideIcon: true, HostSnapshotId: token, Actions: []ToolbarMsgAction{{Id: "run"}}}
	}
	manager.ShowToolbarMsg(ctx, a, msg("first", "first-token"))
	manager.presentation.selectResultContext(ctx, "window", "query", "second", b.Metadata.Id)
	manager.ShowToolbarMsg(ctx, b, msg("second", "queued-old"))
	manager.ShowToolbarMsg(ctx, b, msg("second", "queued-new"))
	select {
	case ids := <-host.released:
		require.Equal(t, "queued-old", ids)
	case <-time.After(time.Second):
		t.Fatal("replaced queued message did not release callbacks")
	}
	manager.ClearToolbarMsg(ctx, b, "second")
	select {
	case ids := <-host.released:
		require.Equal(t, "queued-new", ids)
	case <-time.After(time.Second):
		t.Fatal("cleared queued message did not release callbacks")
	}
	expiry()
	hold()
	select {
	case ids := <-host.released:
		require.Equal(t, "first-token", ids)
	case <-time.After(time.Second):
		t.Fatal("expired message did not release callbacks")
	}
}

// TestPendingToolbarSnapshotPromotesAndClosesWithSession keeps only the selected message's callbacks.
func TestPendingToolbarSnapshotPromotesAndClosesWithSession(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
	manager.AttachUI(&presentationTestUI{})
	var hold func()
	manager.presentation.scheduleToolbarHold = func(_ time.Duration, callback func()) { hold = callback }
	host := &toolbarCallbackTestPlugin{released: make(chan string, 4), started: make(chan string, 1), finish: make(chan struct{})}
	close(host.finish)
	a, b := newInitTestInstance(host), newInitTestInstance(host)
	a.Metadata.Id, b.Metadata.Id = "first", "second"
	manager.lifecycle.appendPluginInstance(a)
	manager.lifecycle.appendPluginInstance(b)
	ctx := util.WithSessionContext(context.Background(), "window")
	manager.BeginSessionQuery("window", "query")
	manager.HandleQueryLifecycle(ctx, Query{Id: "query", SessionId: "window", Type: QueryTypeSelection}, nil)
	manager.presentation.selectResultContext(ctx, "window", "query", "first", a.Metadata.Id)
	manager.ShowToolbarMsg(ctx, a, ToolbarMsg{Id: "old", HideIcon: true, IsPersist: true, HostSnapshotId: "old", Actions: []ToolbarMsgAction{{Id: "run"}}})
	manager.presentation.selectResultContext(ctx, "window", "query", "second", b.Metadata.Id)
	manager.ShowToolbarMsg(ctx, b, ToolbarMsg{Id: "new", HideIcon: true, IsPersist: true, HostSnapshotId: "new", Actions: []ToolbarMsgAction{{Id: "run"}}})
	hold()
	select {
	case id := <-host.released:
		require.Equal(t, "old", id)
	case <-time.After(time.Second):
		t.Fatal("previous snapshot was not retired")
	}
	require.NoError(t, manager.ExecuteToolbarMsgAction(ctx, "window", "new", "run"))
	require.Equal(t, "new", <-host.started)
	manager.ClearSessionState(ctx, "window")
	select {
	case id := <-host.released:
		require.Equal(t, "new", id)
	case <-time.After(time.Second):
		t.Fatal("session close did not retire the current snapshot")
	}
}

// TestManagedToolbarCallbackExpiryIsSessionScoped protects a replacement and another window from old timers.
func TestManagedToolbarCallbackExpiryIsSessionScoped(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
	manager.AttachUI(&presentationTestUI{})
	var timers []func()
	manager.presentation.scheduleToolbarExpiry = func(_ time.Duration, callback func()) { timers = append(timers, callback) }
	host := &toolbarCallbackTestPlugin{released: make(chan string, 10)}
	instance := newInitTestInstance(host)
	instance.Metadata.Id = "owner"
	manager.lifecycle.appendPluginInstance(instance)
	for _, sessionID := range []string{"first", "second"} {
		manager.BeginSessionQuery(sessionID, "query")
		manager.HandleQueryLifecycle(context.Background(), Query{Id: "query", SessionId: sessionID, Type: QueryTypeSelection}, nil)
		manager.presentation.selectResultContext(context.Background(), sessionID, "query", "result", instance.Metadata.Id)
	}
	message := func(token string) ToolbarMsg {
		return ToolbarMsg{Id: "status", HideIcon: true, HostSnapshotId: token, Actions: []ToolbarMsgAction{{Id: "run"}}}
	}
	first := util.WithSessionContext(context.Background(), "first")
	second := util.WithSessionContext(context.Background(), "second")
	manager.ShowToolbarMsg(first, instance, message("first-old"))
	manager.ShowToolbarMsg(second, instance, message("second"))
	manager.ShowToolbarMsg(first, instance, message("first-new"))
	select {
	case ids := <-host.released:
		require.Equal(t, "first-old", ids)
	case <-time.After(time.Second):
		t.Fatal("replacement did not release the previous callback")
	}
	timers[0]()
	select {
	case ids := <-host.released:
		t.Fatalf("old timer released a current callback: %v", ids)
	default:
	}
	timers[2]()
	select {
	case ids := <-host.released:
		require.Equal(t, "first-new", ids)
	case <-time.After(time.Second):
		t.Fatal("current callback was not released")
	}
	_, found := manager.presentation.pluginToolbarMsgIds.Load(toolbarSessionKey("second", "owner"))
	require.True(t, found)
	manager.ClearToolbarMsg(second, instance, "status")
	select {
	case ids := <-host.released:
		require.Equal(t, "second", ids)
	case <-time.After(time.Second):
		t.Fatal("other window callback was not released")
	}
}

// TestManagedToolbarCallbacksCannotReplayAfterUnload protects a queued snapshot racing plugin cleanup.
func TestManagedToolbarCallbacksCannotReplayAfterUnload(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
	ui := &presentationTestUI{}
	manager.AttachUI(ui)
	host := &toolbarCallbackTestPlugin{released: make(chan string, 2)}
	instance := newInitTestInstance(host)
	instance.Metadata.Id = "owner"
	manager.lifecycle.appendPluginInstance(instance)
	ctx := util.WithQueryIdContext(util.WithSessionContext(context.Background(), "window"), "query")
	manager.BeginSessionQuery("window", "query")
	manager.HandleQueryLifecycle(ctx, Query{Id: "query", SessionId: "window", Type: QueryTypeSelection}, nil)
	manager.presentation.selectResultContext(ctx, "window", "query", "result", instance.Metadata.Id)
	epoch := instance.toolbarEpoch.Load()
	manager.presentation.releasePlugin(instance.Metadata.Id)
	manager.presentation.showSessionToolbarMsg(ctx, instance, ToolbarMsg{Id: "stale", HideIcon: true, HostSnapshotId: "stale-token", Actions: []ToolbarMsgAction{{Id: "run"}}}, true, true, epoch)
	require.Empty(t, ui.shown)
	select {
	case ids := <-host.released:
		require.Equal(t, "stale-token", ids)
	case <-time.After(time.Second):
		t.Fatal("retired queued callback was not released")
	}
}
