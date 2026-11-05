package plugin

import (
	"context"
	"errors"
	"testing"
	"time"

	"wox/common"
	"wox/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type presentationTestUI struct {
	common.UI
	shown   []ToolbarMsgUI
	cleared []string
}

func (ui *presentationTestUI) ShowToolbarMsg(_ context.Context, msg common.ToolbarMsgUI) {
	ui.shown = append(ui.shown, msg)
}

func (ui *presentationTestUI) ClearToolbarMsg(_ context.Context, id string) {
	ui.cleared = append(ui.cleared, id)
}

// TestToolbarMessageExpiry protects replacement snapshots and persistent messages from old timers.
func TestToolbarMessageExpiry(t *testing.T) {
	manager := newManager()
	ui := &presentationTestUI{}
	manager.AttachUI(ui)
	instance := newInitTestInstance(&fakeLifecyclePlugin{})
	instance.Metadata.Id = "expiry"
	ctx := util.WithSessionContext(context.Background(), "one")
	otherCtx := util.WithSessionContext(context.Background(), "two")
	var timers []func()
	manager.presentation.scheduleToolbarExpiry = func(delay time.Duration, expire func()) {
		require.Equal(t, 10*time.Second, delay)
		timers = append(timers, expire)
	}
	message := ToolbarMsg{Id: "status", Actions: []ToolbarMsgAction{{Id: "run", Action: func(context.Context, ToolbarMsgActionContext) {}}}}
	manager.ShowSessionToolbarMsg(ctx, instance, message)
	manager.ShowSessionToolbarMsg(otherCtx, instance, message)
	require.Len(t, timers, 2)

	// Updating the same ID restarts its lifetime; an earlier timer cannot clear it.
	manager.ShowSessionToolbarMsg(ctx, instance, message)
	require.Len(t, timers, 3)
	timers[0]()
	require.Empty(t, ui.cleared)
	require.NoError(t, manager.ExecuteToolbarMsgAction(ctx, "one", "status", "run"))
	timers[2]()
	require.Equal(t, []string{"status"}, ui.cleared)
	require.Error(t, manager.ExecuteToolbarMsgAction(ctx, "one", "status", "run"))
	require.NoError(t, manager.ExecuteToolbarMsgAction(ctx, "two", "status", "run"))

	message.IsPersist = true
	manager.ShowSessionToolbarMsg(otherCtx, instance, message)
	require.Len(t, timers, 3)
	timers[1]()
	require.Len(t, ui.cleared, 1)
	require.NoError(t, manager.ExecuteToolbarMsgAction(ctx, "two", "status", "run"))
	manager.ClearToolbarMsg(otherCtx, instance, "status")
	require.Equal(t, []string{"status", "status"}, ui.cleared)
	require.Error(t, manager.ExecuteToolbarMsgAction(ctx, "two", "status", "run"))
}

// TestPresentationCallbacksFollowPluginLifetime exercises the public Manager wiring and owner isolation.
func TestPresentationCallbacksFollowPluginLifetime(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
	ui := &presentationTestUI{}
	manager.AttachUI(ui)
	ctx := util.WithSessionContext(context.Background(), "session")
	for _, id := range []string{"first", "first-other"} {
		instance := newInitTestInstance(&fakeLifecyclePlugin{})
		instance.Metadata.Id = id
		manager.lifecycle.appendPluginInstance(instance)
		manager.presentation.sessionPluginQueries.Store(id, &sessionPluginQueryState{PluginId: id})
		manager.ShowToolbarMsg(util.WithSessionContext(ctx, id), instance, ToolbarMsg{Id: id, Actions: []ToolbarMsgAction{{Id: "run", Action: func(context.Context, ToolbarMsgActionContext) {}}}})
		manager.presentation.normalizeGlanceItem(ctx, instance, GlanceItem{Id: "status", Action: &GlanceAction{Id: "run", Action: func(context.Context, GlanceActionContext) {}}})
	}
	require.Len(t, ui.shown, 2)
	require.NoError(t, manager.ExecuteToolbarMsgAction(ctx, "first", "first", "run"))
	require.NoError(t, manager.ExecuteGlanceAction(ctx, "first", "status", "run"))
	manager.UnloadPlugin(ctx, manager.GetPluginInstanceById("first"))
	require.Equal(t, []string{"first"}, ui.cleared)
	require.Error(t, manager.ExecuteToolbarMsgAction(ctx, "first", "first", "run"))
	require.Error(t, manager.ExecuteGlanceAction(ctx, "first", "status", "run"))
	require.NoError(t, manager.ExecuteToolbarMsgAction(ctx, "first-other", "first-other", "run"))
	require.NoError(t, manager.ExecuteGlanceAction(ctx, "first-other", "status", "run"))
	ownerMsg, ok := manager.presentation.pluginToolbarMsgIds.Load(toolbarSessionKey("first-other", "first-other"))
	assert.True(t, ok)
	assert.Equal(t, "first-other", ownerMsg.MessageId)
	assert.Equal(t, 1, manager.presentation.pluginToolbarMsgIds.Len())

	// A failed re-initialization must also drop callbacks registered by that lifecycle.
	other := manager.GetPluginInstanceById("first-other")
	other.Plugin = &fakeLifecyclePlugin{initErr: errors.New("init failed")}
	manager.lifecycle.initPlugin(ctx, other)
	require.Equal(t, []string{"first", "first-other"}, ui.cleared)
	require.Error(t, manager.ExecuteGlanceAction(ctx, "first-other", "status", "run"))
	require.Error(t, manager.ExecuteToolbarMsgAction(ctx, "first-other", "first-other", "run"))
}

func TestSessionToolbarMessageActionsWithoutScopedQuery(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
	ui := &presentationTestUI{}
	manager.AttachUI(ui)
	instance := newInitTestInstance(&fakeLifecyclePlugin{})
	instance.Metadata.Id = "sys-preview"
	called := false
	msg := ToolbarMsg{Id: "preview", Title: "Long message", Actions: []ToolbarMsgAction{{Id: "run", Name: "Run", Action: func(context.Context, ToolbarMsgActionContext) { called = true }}}}
	manager.ShowSessionToolbarMsg(context.Background(), instance, msg)
	require.Empty(t, ui.shown)
	ctx := util.WithQueryIdContext(util.WithSessionContext(context.Background(), "session"), "query")
	manager.ShowSessionToolbarMsg(ctx, instance, msg)
	require.Len(t, ui.shown, 1)
	require.NoError(t, manager.ExecuteToolbarMsgAction(ctx, "session", "preview", "run"))
	require.True(t, called)
	manager.ClearToolbarMsg(ctx, instance, "preview")
	require.Error(t, manager.ExecuteToolbarMsgAction(ctx, "session", "preview", "run"))
}

// TestGlobalResultToolbarContext retains status across one plugin's rows and isolates sessions.
func TestGlobalResultToolbarContext(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
	var holds []func()
	manager.presentation.scheduleToolbarHold = func(_ time.Duration, expire func()) { holds = append(holds, expire) }
	ui := &presentationTestUI{}
	manager.AttachUI(ui)
	instance := newInitTestInstance(&fakeLifecyclePlugin{})
	instance.Metadata.Id = "global-only"
	instance.Metadata.TriggerKeywords = []string{"*"}
	entered := make(chan context.Context, 8)
	instance.EnterPluginQueryCallbacks = []func(context.Context){func(ctx context.Context) { entered <- ctx }}
	manager.lifecycle.appendPluginInstance(instance)
	ctx := context.Background()
	for _, session := range []string{"one", "two"} {
		manager.BeginSessionQuery(session, "query")
		manager.HandleQueryLifecycle(ctx, Query{Id: "query", SessionId: session, Type: QueryTypeInput}, nil)
		manager.presentation.selectResultContext(ctx, session, "query", "result", instance.Metadata.Id)
		manager.presentation.allowGlobalToolbarForAction(session, "query", instance.Metadata.Id)
	}
	first, second := <-entered, <-entered
	if util.GetContextSessionId(first) != "one" {
		first, second = second, first
	}
	require.Equal(t, "one", util.GetContextSessionId(first))
	require.Equal(t, "two", util.GetContextSessionId(second))
	calls := []string{}
	for _, selected := range []context.Context{first, second} {
		session := util.GetContextSessionId(selected)
		manager.ShowToolbarMsg(selected, instance, ToolbarMsg{Id: "same-id", Title: session, Actions: []ToolbarMsgAction{{Id: "run", Action: func(context.Context, ToolbarMsgActionContext) { calls = append(calls, session) }}}})
	}
	require.Len(t, ui.shown, 2)
	manager.ShowToolbarMsg(ctx, instance, ToolbarMsg{Id: "unscoped", Title: "ambiguous session"})
	require.Len(t, ui.shown, 2)
	require.NoError(t, manager.ExecuteToolbarMsgAction(ctx, "one", "same-id", "run"))
	require.NoError(t, manager.ExecuteToolbarMsgAction(ctx, "two", "same-id", "run"))
	require.Equal(t, []string{"one", "two"}, calls)
	oldSnapshot := ui.shown[0]
	manager.presentation.selectResultContext(ctx, "one", "query", "result-two", instance.Metadata.Id)
	require.True(t, oldSnapshot.IsCurrent())
	require.NoError(t, manager.ExecuteToolbarMsgAction(ctx, "one", "same-id", "run"))
	require.Empty(t, ui.cleared)
	select {
	case <-entered:
		t.Fatal("switching rows of one plugin re-entered its context")
	default:
	}
	manager.ShowToolbarMsg(first, instance, ToolbarMsg{Id: "same-id", Title: "second result"})
	require.Len(t, ui.shown, 3)
	manager.presentation.selectResultContext(ctx, "one", "query", "other", "other-plugin")
	require.True(t, ui.shown[2].IsCurrent())
	holds[0]()
	require.False(t, ui.shown[2].IsCurrent())
	require.Equal(t, []string{"same-id"}, ui.cleared)
	require.Error(t, manager.ExecuteToolbarMsgAction(ctx, "one", "same-id", "run"))
	require.NoError(t, manager.ExecuteToolbarMsgAction(ctx, "two", "same-id", "run"))
	manager.ShowToolbarMsg(first, instance, ToolbarMsg{Id: "late", Title: "stale"})
	require.Len(t, ui.shown, 3)
	manager.presentation.selectResultContext(ctx, "one", "query", "result", instance.Metadata.Id)
	fresh := <-entered
	manager.presentation.allowGlobalToolbarForAction("one", "query", instance.Metadata.Id)
	// A previous callback from the same query is accepted after returning to its plugin.
	manager.ShowToolbarMsg(first, instance, ToolbarMsg{Id: "same-id", Title: "late from A"})
	require.Len(t, ui.shown, 4)
	manager.ClearToolbarMsg(first, instance, "same-id")
	require.False(t, ui.shown[3].IsCurrent())
	manager.ShowToolbarMsg(fresh, instance, ToolbarMsg{Id: "same-id", Title: "working again"})
	require.Len(t, ui.shown, 5)
	manager.presentation.selectResultContext(ctx, "one", "query", "", "")
	holds[1]()
	require.False(t, ui.shown[4].IsCurrent())
	require.Equal(t, []string{"same-id", "same-id", "same-id"}, ui.cleared)
	manager.presentation.selectResultContext(ctx, "one", "query", "result", instance.Metadata.Id)
	fresh = <-entered
	manager.presentation.allowGlobalToolbarForAction("one", "query", instance.Metadata.Id)
	manager.ShowToolbarMsg(fresh, instance, ToolbarMsg{Id: "same-id", Title: "selected again"})
	require.Len(t, ui.shown, 6)
	manager.BeginSessionQuery("one", "next")
	manager.ShowToolbarMsg(fresh, instance, ToolbarMsg{Id: "late", Title: "stale query"})
	require.Len(t, ui.shown, 7)
	manager.presentation.selectResultContext(ctx, "one", "query", "other", "")
	require.True(t, manager.presentation.isPluginActiveInSession("one", instance.Metadata.Id))
}

// TestSelectedFileResultAndScopedQueryOwnToolbar covers both result-selected and fixed query owners.
func TestSelectedFileResultAndScopedQueryOwnToolbar(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
	var holds []func()
	manager.presentation.scheduleToolbarHold = func(_ time.Duration, expire func()) { holds = append(holds, expire) }
	ui := &presentationTestUI{}
	manager.AttachUI(ui)
	instance := newInitTestInstance(&fakeLifecyclePlugin{})
	instance.Metadata.Id = "installer"
	manager.lifecycle.appendPluginInstance(instance)
	entered := make(chan context.Context, 3)
	instance.EnterPluginQueryCallbacks = []func(context.Context){func(ctx context.Context) { entered <- ctx }}
	ctx := context.Background()
	manager.BeginSessionQuery("window", "selection")
	manager.HandleQueryLifecycle(ctx, Query{Id: "selection", SessionId: "window", Type: QueryTypeSelection}, nil)
	manager.presentation.selectResultContext(ctx, "window", "selection", "local-package", instance.Metadata.Id)
	selected := <-entered
	require.Equal(t, "local-package", manager.SelectedResultID(selected, instance.Metadata.Id))
	require.Empty(t, manager.SelectedResultID(selected, "other-plugin"))
	manager.ShowToolbarMsg(selected, instance, ToolbarMsg{Id: "install", Title: "Installing local-package"})
	require.Len(t, ui.shown, 1)
	manager.presentation.selectResultContext(ctx, "window", "selection", "other-package", instance.Metadata.Id)
	require.Equal(t, "other-package", manager.SelectedResultID(selected, instance.Metadata.Id))
	require.True(t, ui.shown[0].IsCurrent())
	manager.presentation.selectResultContext(ctx, "window", "selection", "different", "")
	require.Empty(t, manager.SelectedResultID(selected, instance.Metadata.Id))
	holds[0]()
	require.Equal(t, []string{"install"}, ui.cleared)

	manager.BeginSessionQuery("window", "scoped-one")
	manager.HandleQueryLifecycle(ctx, Query{Id: "scoped-one", SessionId: "window", Type: QueryTypeInput, TriggerKeyword: "installer"}, instance)
	scoped := <-entered
	manager.ShowToolbarMsg(scoped, instance, ToolbarMsg{Id: "scope", Title: "Working"})
	manager.BeginSessionQuery("window", "scoped-two")
	manager.HandleQueryLifecycle(ctx, Query{Id: "scoped-two", SessionId: "window", Type: QueryTypeInput, TriggerKeyword: "installer"}, instance)
	require.True(t, ui.shown[1].IsCurrent(), "visible message survives a new query in the same plugin context")
	manager.BeginSessionQuery("window", "other-query")
	manager.HandleQueryLifecycle(ctx, Query{Id: "other-query", SessionId: "window", Type: QueryTypeInput}, nil)
	require.Equal(t, []string{"install", "scope"}, ui.cleared)
}

// TestToolbarSelectionHold covers refresh, replacement, timeout, and session isolation.
func TestToolbarSelectionHold(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
	ui := &presentationTestUI{}
	manager.AttachUI(ui)
	var holds []func()
	manager.presentation.scheduleToolbarHold = func(delay time.Duration, expire func()) {
		require.Equal(t, toolbarSelectionHoldDuration, delay)
		holds = append(holds, expire)
	}
	a, b := newInitTestInstance(&fakeLifecyclePlugin{}), newInitTestInstance(&fakeLifecyclePlugin{})
	a.Metadata.Id, b.Metadata.Id = "installer", "other"
	manager.lifecycle.appendPluginInstance(a)
	manager.lifecycle.appendPluginInstance(b)
	ctx := context.Background()
	for _, session := range []string{"one", "two"} {
		manager.BeginSessionQuery(session, "first")
		manager.HandleQueryLifecycle(ctx, Query{Id: "first", SessionId: session, Type: QueryTypeSelection}, nil)
		manager.presentation.selectResultContext(ctx, session, "first", "file", a.Metadata.Id)
		manager.ShowToolbarMsg(util.WithQueryIdContext(util.WithSessionContext(ctx, session), "first"), a, ToolbarMsg{Id: "install", Title: session, IsPersist: true})
	}
	oldCtx := util.WithQueryIdContext(util.WithSessionContext(ctx, "one"), "first")
	manager.BeginSessionQuery("one", "refresh")
	manager.HandleQueryLifecycle(ctx, Query{Id: "refresh", SessionId: "one", Type: QueryTypeSelection}, nil)
	require.True(t, ui.shown[0].IsCurrent())
	require.Empty(t, ui.cleared)
	manager.ShowToolbarMsg(oldCtx, a, ToolbarMsg{Id: "install", Title: "completed", IsPersist: true})
	require.Equal(t, "completed", ui.shown[2].Title)
	manager.presentation.selectResultContext(ctx, "one", "refresh", "file-new", a.Metadata.Id)
	holds[0]()
	require.True(t, ui.shown[2].IsCurrent(), "old timer must not clear a restored owner")
	require.True(t, ui.shown[1].IsCurrent(), "other session remains untouched")
	require.Empty(t, ui.cleared)

	manager.presentation.selectResultContext(ctx, "one", "refresh", "other", b.Metadata.Id)
	manager.ShowToolbarMsg(util.WithSessionContext(ctx, "one"), b, ToolbarMsg{Id: "other-status", Title: "new owner", IsPersist: true})
	require.Len(t, ui.shown, 3, "new owner is held until the deadline")
	require.True(t, ui.shown[2].IsCurrent())
	manager.ClearToolbarMsg(util.WithSessionContext(ctx, "one"), b, "other-status")
	manager.BeginSessionQuery("one", "next")
	manager.HandleQueryLifecycle(ctx, Query{Id: "next", SessionId: "one", Type: QueryTypeSelection}, nil)
	manager.presentation.selectResultContext(ctx, "one", "next", "other-new", b.Metadata.Id)
	manager.ShowToolbarMsg(util.WithSessionContext(ctx, "one"), b, ToolbarMsg{Id: "other-status", Title: "latest", IsPersist: true})
	require.Len(t, holds, 2, "refreshes cannot extend the original deadline")
	holds[1]()
	require.Equal(t, []string{"install"}, ui.cleared)
	require.Len(t, ui.shown, 4)
	require.Equal(t, "latest", ui.shown[3].Title)
	require.True(t, ui.shown[3].IsCurrent())
	require.True(t, ui.shown[1].IsCurrent())

	manager.presentation.selectResultContext(ctx, "one", "next", "", "")
	require.True(t, ui.shown[3].IsCurrent())
	holds[2]()
	require.False(t, ui.shown[3].IsCurrent())
	require.Equal(t, []string{"install", "other-status"}, ui.cleared)
}

// TestToolbarSelectionHoldDoesNotReplayClearedOrClosedSessions protects deferred messages.
func TestToolbarSelectionHoldDoesNotReplayClearedOrClosedSessions(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
	ui := &presentationTestUI{}
	manager.AttachUI(ui)
	var expire func()
	manager.presentation.scheduleToolbarHold = func(_ time.Duration, callback func()) { expire = callback }
	a, b := newInitTestInstance(&fakeLifecyclePlugin{}), newInitTestInstance(&fakeLifecyclePlugin{})
	a.Metadata.Id, b.Metadata.Id = "first", "second"
	manager.lifecycle.appendPluginInstance(a)
	manager.lifecycle.appendPluginInstance(b)
	ctx := util.WithSessionContext(context.Background(), "window")
	manager.BeginSessionQuery("window", "query")
	manager.HandleQueryLifecycle(ctx, Query{Id: "query", SessionId: "window", Type: QueryTypeSelection}, nil)
	manager.presentation.selectResultContext(ctx, "window", "query", "first", a.Metadata.Id)
	manager.ShowToolbarMsg(ctx, a, ToolbarMsg{Id: "first", IsPersist: true})
	manager.presentation.selectResultContext(ctx, "window", "query", "second", b.Metadata.Id)
	manager.ShowToolbarMsg(ctx, b, ToolbarMsg{Id: "second", IsPersist: true})
	manager.ClearToolbarMsg(ctx, b, "second")
	expire()
	require.Len(t, ui.shown, 1, "a cleared deferred message must not reappear")
	require.Equal(t, []string{"first"}, ui.cleared)

	manager.ShowToolbarMsg(ctx, b, ToolbarMsg{Id: "second", IsPersist: true})
	manager.presentation.selectResultContext(ctx, "window", "query", "first", a.Metadata.Id)
	manager.ShowToolbarMsg(ctx, a, ToolbarMsg{Id: "first", IsPersist: true})
	manager.BeginSessionQuery("window", "empty")
	manager.HandleQueryLifecycle(ctx, Query{Id: "empty", SessionId: "window", Type: QueryTypeSelection}, nil)
	manager.ClearSessionState(ctx, "window")
	expire()
	require.Len(t, ui.shown, 2, "destroying a session must discard its deferred message")
	require.Equal(t, []string{"first", "second"}, ui.cleared)
}

// TestToolbarSelectionHoldDiscardsUnloadedPlugin checks the deferred callback lifetime.
func TestToolbarSelectionHoldDiscardsUnloadedPlugin(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
	ui := &presentationTestUI{}
	manager.AttachUI(ui)
	var expire func()
	manager.presentation.scheduleToolbarHold = func(_ time.Duration, callback func()) { expire = callback }
	a, b := newInitTestInstance(&fakeLifecyclePlugin{}), newInitTestInstance(&fakeLifecyclePlugin{})
	a.Metadata.Id, b.Metadata.Id = "first", "second"
	manager.lifecycle.appendPluginInstance(a)
	manager.lifecycle.appendPluginInstance(b)
	ctx := util.WithSessionContext(context.Background(), "window")
	manager.BeginSessionQuery("window", "query")
	manager.HandleQueryLifecycle(ctx, Query{Id: "query", SessionId: "window", Type: QueryTypeSelection}, nil)
	manager.presentation.selectResultContext(ctx, "window", "query", "first", a.Metadata.Id)
	manager.ShowToolbarMsg(ctx, a, ToolbarMsg{Id: "first", IsPersist: true})
	manager.presentation.selectResultContext(ctx, "window", "query", "second", b.Metadata.Id)
	manager.ShowToolbarMsg(ctx, b, ToolbarMsg{Id: "second", IsPersist: true})
	manager.presentation.releasePlugin(b.Metadata.Id)
	expire()
	require.Len(t, ui.shown, 1)
	require.Equal(t, []string{"first"}, ui.cleared)
}

// TestUnscopedToolbarMessageUsesOnlyUnambiguousActivePluginSession covers background status calls.
func TestUnscopedToolbarMessageUsesOnlyUnambiguousActivePluginSession(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
	ui := &presentationTestUI{}
	manager.AttachUI(ui)
	instance := newInitTestInstance(&fakeLifecyclePlugin{})
	instance.Metadata.Id = "global-only"
	manager.BeginSessionQuery("window", "query")
	manager.HandleQueryLifecycle(context.Background(), Query{Id: "query", SessionId: "window", Type: QueryTypeInput}, nil)
	manager.presentation.selectResultContext(context.Background(), "window", "query", "result", instance.Metadata.Id)
	manager.presentation.allowGlobalToolbarForAction("window", "query", instance.Metadata.Id)
	manager.ShowToolbarMsg(context.Background(), instance, ToolbarMsg{Id: "background", Title: "Plugin status"})
	require.Len(t, ui.shown, 1)
	require.True(t, ui.shown[0].IsCurrent())
}

// TestGlobalToolbarRequiresResultAction checks real action execution and session-scoped eligibility.
func TestGlobalToolbarRequiresResultAction(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
	ui := &presentationTestUI{}
	manager.AttachUI(ui)
	instance := newInitTestInstance(&fakeLifecyclePlugin{})
	instance.Metadata.Id = "file-search"
	manager.lifecycle.appendPluginInstance(instance)
	query := Query{Id: "query", SessionId: "one", Type: QueryTypeInput}
	manager.BeginSessionQuery(query.SessionId, query.Id)
	manager.HandleQueryLifecycle(context.Background(), query, nil)
	manager.presentation.selectResultContext(context.Background(), query.SessionId, query.Id, "result", instance.Metadata.Id)
	ctx := util.WithQueryIdContext(util.WithSessionContext(context.Background(), "one"), "query")
	manager.ShowToolbarMsg(ctx, instance, ToolbarMsg{Id: "status", Title: "background", IsPersist: true})
	manager.ShowToolbarMsg(context.Background(), instance, ToolbarMsg{Id: "status", Title: "unscoped", IsPersist: true})
	require.Empty(t, ui.shown, "selection and background callbacks alone stay quiet")

	manager.resultStore.startQuery(query)
	set, ok := manager.resultStore.getQuery(query.SessionId, query.Id)
	require.True(t, ok)
	result := QueryResult{Id: "result", Actions: []QueryResultAction{{Id: systemActionPinInQueryID, Action: func(actionCtx context.Context, _ ActionContext) {
		manager.ShowToolbarMsg(actionCtx, instance, ToolbarMsg{Id: "status", Title: "action completed", IsPersist: true})
	}}}}
	set.Results.Store(result.Id, &QueryResultCache{Result: result, Query: query, PluginInstance: instance})
	require.Error(t, manager.ExecuteAction(ctx, "one", "query", "result", "missing"))
	manager.ShowToolbarMsg(ctx, instance, ToolbarMsg{Id: "status", Title: "still blocked", IsPersist: true})
	require.Empty(t, ui.shown, "invalid actions must not enable messages")
	require.NoError(t, manager.ExecuteAction(ctx, "one", "query", "result", systemActionPinInQueryID))
	require.Len(t, ui.shown, 1)
	require.Equal(t, "action completed", ui.shown[0].Title)

	manager.BeginSessionQuery("two", "query")
	manager.HandleQueryLifecycle(context.Background(), Query{Id: "query", SessionId: "two", Type: QueryTypeInput}, nil)
	manager.presentation.selectResultContext(context.Background(), "two", "query", "result", instance.Metadata.Id)
	manager.ShowToolbarMsg(util.WithSessionContext(context.Background(), "two"), instance, ToolbarMsg{Id: "status", Title: "other window"})
	require.Len(t, ui.shown, 1, "an action in another window grants no permission")

	formQuery := Query{Id: "query", SessionId: "two", Type: QueryTypeInput}
	manager.resultStore.startQuery(formQuery)
	formSet, ok := manager.resultStore.getQuery("two", "query")
	require.True(t, ok)
	form := QueryResult{Id: "result", Actions: []QueryResultAction{{Id: systemActionPinInQueryID, Type: QueryResultActionTypeForm, OnSubmit: func(actionCtx context.Context, _ FormActionContext) {
		manager.ShowToolbarMsg(actionCtx, instance, ToolbarMsg{Id: "status", Title: "form submitted", IsPersist: true})
	}}}}
	formSet.Results.Store(form.Id, &QueryResultCache{Result: form, Query: formQuery, PluginInstance: instance})
	require.NoError(t, manager.SubmitFormAction(context.Background(), "two", "query", "result", systemActionPinInQueryID, nil))
	require.Len(t, ui.shown, 2)
	require.Equal(t, "form submitted", ui.shown[1].Title)
}

// TestGlobalToolbarActionPermissionSurvivesRefreshOnlyForTheSamePlugin protects long-running actions.
func TestGlobalToolbarActionPermissionSurvivesRefreshOnlyForTheSamePlugin(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
	ui := &presentationTestUI{}
	manager.AttachUI(ui)
	var expire func()
	manager.presentation.scheduleToolbarHold = func(_ time.Duration, callback func()) { expire = callback }
	a, b := newInitTestInstance(&fakeLifecyclePlugin{}), newInitTestInstance(&fakeLifecyclePlugin{})
	a.Metadata.Id, b.Metadata.Id = "first", "second"
	ctx := util.WithQueryIdContext(util.WithSessionContext(context.Background(), "window"), "old")
	manager.BeginSessionQuery("window", "old")
	manager.HandleQueryLifecycle(ctx, Query{Id: "old", SessionId: "window", Type: QueryTypeInput}, nil)
	manager.presentation.selectResultContext(ctx, "window", "old", "first", a.Metadata.Id)
	manager.presentation.allowGlobalToolbarForAction("window", "old", a.Metadata.Id)
	manager.BeginSessionQuery("window", "refresh")
	manager.HandleQueryLifecycle(ctx, Query{Id: "refresh", SessionId: "window", Type: QueryTypeInput}, nil)
	manager.ShowToolbarMsg(ctx, a, ToolbarMsg{Id: "status", Title: "in progress", IsPersist: true})
	require.Len(t, ui.shown, 1, "an action may finish while refreshed results are pending")
	manager.presentation.selectResultContext(ctx, "window", "refresh", "first-new", a.Metadata.Id)
	expire()
	manager.ShowToolbarMsg(ctx, a, ToolbarMsg{Id: "status", Title: "completed", IsPersist: true})
	require.Equal(t, "completed", ui.shown[1].Title)
	manager.presentation.selectResultContext(ctx, "window", "refresh", "other", b.Metadata.Id)
	manager.ShowToolbarMsg(util.WithSessionContext(context.Background(), "window"), b, ToolbarMsg{Id: "other", Title: "uninvoked"})
	require.Len(t, ui.shown, 2)
	expire()
	manager.presentation.selectResultContext(ctx, "window", "refresh", "first", a.Metadata.Id)
	manager.ShowToolbarMsg(ctx, a, ToolbarMsg{Id: "status", Title: "old permission"})
	require.Len(t, ui.shown, 2, "returning after leaving requires another action")
}

// TestGlobalToolbarQueuesActionDuringAnotherOwnersHold covers refresh while the previous toolbar is still visible.
func TestGlobalToolbarQueuesActionDuringAnotherOwnersHold(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
	ui := &presentationTestUI{}
	manager.AttachUI(ui)
	var expire func()
	manager.presentation.scheduleToolbarHold = func(_ time.Duration, callback func()) { expire = callback }
	a, b := newInitTestInstance(&fakeLifecyclePlugin{}), newInitTestInstance(&fakeLifecyclePlugin{})
	a.Metadata.Id, b.Metadata.Id = "first", "second"
	ctx := util.WithQueryIdContext(util.WithSessionContext(context.Background(), "window"), "old")
	manager.BeginSessionQuery("window", "old")
	manager.HandleQueryLifecycle(ctx, Query{Id: "old", SessionId: "window", Type: QueryTypeInput}, nil)
	manager.presentation.selectResultContext(ctx, "window", "old", "first", a.Metadata.Id)
	manager.presentation.allowGlobalToolbarForAction("window", "old", a.Metadata.Id)
	manager.ShowToolbarMsg(ctx, a, ToolbarMsg{Id: "first", Title: "old", IsPersist: true})
	manager.presentation.selectResultContext(ctx, "window", "old", "second", b.Metadata.Id)
	manager.presentation.allowGlobalToolbarForAction("window", "old", b.Metadata.Id)
	manager.ShowToolbarMsg(ctx, b, ToolbarMsg{Id: "second", Title: "started", IsPersist: true})
	manager.BeginSessionQuery("window", "refresh")
	manager.HandleQueryLifecycle(ctx, Query{Id: "refresh", SessionId: "window", Type: QueryTypeInput}, nil)
	manager.ShowToolbarMsg(ctx, b, ToolbarMsg{Id: "second", Title: "finished", IsPersist: true})
	manager.presentation.selectResultContext(ctx, "window", "refresh", "second-new", b.Metadata.Id)
	require.Len(t, ui.shown, 1)
	expire()
	require.Len(t, ui.shown, 2)
	require.Equal(t, "finished", ui.shown[1].Title)
}
