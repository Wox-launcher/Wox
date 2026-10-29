package plugin

import (
	"context"
	"testing"
	"time"
	"wox/common"
	"wox/util"

	"github.com/stretchr/testify/require"
)

type mruLifecycleTestUI struct {
	presentationTestUI
	beforeRestore func()
}

func (ui *mruLifecycleTestUI) GetActiveWindowSnapshot(context.Context) common.ActiveWindowSnapshot {
	ui.beforeRestore()
	return common.ActiveWindowSnapshot{}
}

// TestQueryMRULeavesPluginContext exercises the actual homepage entry point:
// restoring recent results must release the previous plugin before any restore work.
func TestQueryMRULeavesPluginContext(t *testing.T) {
	initPluginManagerLoadTest(t)
	manager := newManager()
	ui := &mruLifecycleTestUI{}
	manager.AttachUI(ui)
	ctx := util.WithSessionContext(context.Background(), "launcher")
	instance := newInitTestInstance(&fakeLifecyclePlugin{})
	instance.Metadata.Id = common.FileSearchPluginID
	left := make(chan struct{}, 2)
	instance.LeavePluginQueryCallbacks = []func(context.Context){func(context.Context) { left <- struct{}{} }}
	manager.lifecycle.appendPluginInstance(instance)
	fileQuery := Query{Id: "file-query", SessionId: "launcher", Type: QueryTypeInput, TriggerKeyword: "f"}
	manager.HandleQueryLifecycle(ctx, fileQuery, instance)
	count := ToolbarMsg{Id: "file-search-status", Title: "2,313,890 files", HideIcon: true}
	manager.ShowToolbarMsg(ctx, instance, count)
	require.Len(t, ui.shown, 1)

	ui.beforeRestore = func() {
		require.False(t, manager.presentation.isPluginActiveInSession("launcher", instance.Metadata.Id))
		require.Empty(t, manager.VisibleToolbarMsgOwners())
		require.Equal(t, []string{count.Id}, ui.cleared)
	}
	manager.QueryMRU(ctx, "launcher", "home-query")
	select {
	case <-left:
	case <-time.After(time.Second):
		t.Fatal("homepage did not fire the plugin leave callback")
	}
	// A background status update arriving after returning home must be ignored.
	manager.ShowToolbarMsg(ctx, instance, count)
	require.Len(t, ui.shown, 1)
	manager.QueryMRU(ctx, "launcher", "home-refresh")
	require.Equal(t, []string{count.Id}, ui.cleared)

	fileQuery.Id = "file-query-again"
	manager.HandleQueryLifecycle(ctx, fileQuery, instance)
	manager.ShowToolbarMsg(ctx, instance, count)
	require.Len(t, ui.shown, 2)
	manager.HandleQueryLifecycle(ctx, Query{Id: "global-query", SessionId: "launcher", Type: QueryTypeInput}, nil)
	require.Equal(t, []string{count.Id, count.Id}, ui.cleared)
	select {
	case <-left:
	case <-time.After(time.Second):
		t.Fatal("global query did not fire the plugin leave callback")
	}
}
