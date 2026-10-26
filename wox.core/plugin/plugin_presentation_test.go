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
		manager.presentation.sessionPluginQueries.Store("session", &sessionPluginQueryState{PluginId: id})
		manager.ShowToolbarMsg(ctx, instance, ToolbarMsg{Id: id, Actions: []ToolbarMsgAction{{Id: "run", Action: func(context.Context, ToolbarMsgActionContext) {}}}})
		manager.presentation.normalizeGlanceItem(ctx, instance, GlanceItem{Id: "status", Action: &GlanceAction{Id: "run", Action: func(context.Context, GlanceActionContext) {}}})
	}
	require.Len(t, ui.shown, 2)
	require.NoError(t, manager.ExecuteToolbarMsgAction(ctx, "session", "first", "run"))
	require.NoError(t, manager.ExecuteGlanceAction(ctx, "first", "status", "run"))
	manager.UnloadPlugin(ctx, manager.GetPluginInstanceById("first"))
	require.Equal(t, []string{"first"}, ui.cleared)
	require.Error(t, manager.ExecuteToolbarMsgAction(ctx, "session", "first", "run"))
	require.Error(t, manager.ExecuteGlanceAction(ctx, "first", "status", "run"))
	require.NoError(t, manager.ExecuteToolbarMsgAction(ctx, "session", "first-other", "run"))
	require.NoError(t, manager.ExecuteGlanceAction(ctx, "first-other", "status", "run"))
	assert.Equal(t, []string{"first-other=first-other"}, manager.VisibleToolbarMsgOwners())

	// A failed re-initialization must also drop callbacks registered by that lifecycle.
	other := manager.GetPluginInstanceById("first-other")
	other.Plugin = &fakeLifecyclePlugin{initErr: errors.New("init failed")}
	manager.lifecycle.initPlugin(ctx, other)
	require.Equal(t, []string{"first", "first-other"}, ui.cleared)
	require.Error(t, manager.ExecuteGlanceAction(ctx, "first-other", "status", "run"))
	require.Error(t, manager.ExecuteToolbarMsgAction(ctx, "session", "first-other", "run"))
}
