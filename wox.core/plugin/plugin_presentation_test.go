package plugin

import (
	"context"
	"errors"
	"testing"

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

func (ui *presentationTestUI) ShowToolbarMsg(_ context.Context, msg interface{}) {
	ui.shown = append(ui.shown, msg.(ToolbarMsgUI))
}

func (ui *presentationTestUI) ClearToolbarMsg(_ context.Context, id string) {
	ui.cleared = append(ui.cleared, id)
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
	require.False(t, ui.shown[2].IsCurrent())
	require.Equal(t, []string{"same-id"}, ui.cleared)
	require.Error(t, manager.ExecuteToolbarMsgAction(ctx, "one", "same-id", "run"))
	require.NoError(t, manager.ExecuteToolbarMsgAction(ctx, "two", "same-id", "run"))
	manager.ShowToolbarMsg(first, instance, ToolbarMsg{Id: "late", Title: "stale"})
	require.Len(t, ui.shown, 3)
	manager.presentation.selectResultContext(ctx, "one", "query", "result", instance.Metadata.Id)
	fresh := <-entered
	// A previous callback from the same query is accepted after returning to its plugin.
	manager.ShowToolbarMsg(first, instance, ToolbarMsg{Id: "same-id", Title: "late from A"})
	require.Len(t, ui.shown, 4)
	manager.ClearToolbarMsg(first, instance, "same-id")
	require.False(t, ui.shown[3].IsCurrent())
	manager.ShowToolbarMsg(fresh, instance, ToolbarMsg{Id: "same-id", Title: "working again"})
	require.Len(t, ui.shown, 5)
	manager.presentation.selectResultContext(ctx, "one", "query", "", "")
	require.False(t, ui.shown[4].IsCurrent())
	require.Equal(t, []string{"same-id", "same-id", "same-id"}, ui.cleared)
	manager.presentation.selectResultContext(ctx, "one", "query", "result", instance.Metadata.Id)
	fresh = <-entered
	manager.ShowToolbarMsg(fresh, instance, ToolbarMsg{Id: "same-id", Title: "selected again"})
	require.Len(t, ui.shown, 6)
	manager.BeginSessionQuery("one", "next")
	manager.ShowToolbarMsg(fresh, instance, ToolbarMsg{Id: "late", Title: "stale query"})
	require.Len(t, ui.shown, 6)
	manager.presentation.selectResultContext(ctx, "one", "query", "other", "")
	require.True(t, manager.presentation.isPluginActiveInSession("one", instance.Metadata.Id))
}

// TestSelectedFileResultAndScopedQueryOwnToolbar covers both result-selected and fixed query owners.
func TestSelectedFileResultAndScopedQueryOwnToolbar(t *testing.T) {
	logger = util.GetLogger()
	manager := newManager()
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
	require.Equal(t, []string{"install"}, ui.cleared)

	manager.BeginSessionQuery("window", "scoped-one")
	manager.HandleQueryLifecycle(ctx, Query{Id: "scoped-one", SessionId: "window", Type: QueryTypeInput, TriggerKeyword: "installer"}, instance)
	scoped := <-entered
	manager.ShowToolbarMsg(scoped, instance, ToolbarMsg{Id: "scope", Title: "Working"})
	manager.BeginSessionQuery("window", "scoped-two")
	manager.HandleQueryLifecycle(ctx, Query{Id: "scoped-two", SessionId: "window", Type: QueryTypeInput, TriggerKeyword: "installer"}, instance)
	require.True(t, ui.shown[1].IsCurrent(), "visible message survives a new query in the same plugin context")
	require.False(t, ui.shown[1].CanShow(), "an old queued update cannot appear after the query changes")
	manager.BeginSessionQuery("window", "other-query")
	manager.HandleQueryLifecycle(ctx, Query{Id: "other-query", SessionId: "window", Type: QueryTypeInput}, nil)
	require.Equal(t, []string{"install", "scope"}, ui.cleared)
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
	manager.ShowToolbarMsg(context.Background(), instance, ToolbarMsg{Id: "background", Title: "Plugin status"})
	require.Len(t, ui.shown, 1)
	require.True(t, ui.shown[0].IsCurrent())
}
