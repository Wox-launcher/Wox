package plugin

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"wox/common"
	"wox/util"

	"github.com/stretchr/testify/require"
)

type silentLifetimePlugin struct {
	Plugin
	t        *testing.T
	released atomic.Int32
	actions  atomic.Int32
}

// Query models the host's registration of callback cleanup on a managed query.
func (p *silentLifetimePlugin) Query(ctx context.Context, _ Query) QueryResponse {
	register, managed := QueryResultLifetime(ctx)
	if !managed {
		p.t.Error("silent query did not supply managed callback lifetime")
		return QueryResponse{}
	}
	register(p, func() { p.released.Add(1) })
	return QueryResponse{Results: []QueryResult{{
		Id: "row", Title: "Silent result", Icon: common.NewWoxImageEmoji("x"),
		Actions: []QueryResultAction{{
			// Skip unrelated ranking persistence in the asynchronous post-action hook.
			Id: systemActionSetResultHotkeyID, Name: "Run", IsDefault: true,
			Action: func(context.Context, ActionContext) {
				require.Equal(p.t, p.actions.Load(), p.released.Load(), "callbacks released before executing the action")
				p.actions.Add(1)
			},
		}},
	}}}
}

// TestSilentQueryReleasesTemporarySessions exercises repeated hotkeys through the public query path.
func TestSilentQueryReleasesTemporarySessions(t *testing.T) {
	initPluginManagerLoadTest(t)
	manager := newManager()
	manager.autoQueryHistory = nil
	plugin := &silentLifetimePlugin{t: t}
	instance := &Instance{Plugin: plugin, Metadata: Metadata{Id: "silent", Name: "Silent", TriggerKeywords: []string{"silent"}}}
	instance.API = NewAPI(instance)
	require.True(t, manager.lifecycle.appendPluginInstance(instance))
	manager.resultStore.startQuery(Query{SessionId: "visible", Id: "keep"})
	for i := 0; i < 3; i++ {
		ctx := util.WithCoreSessionContext(util.NewTraceContext())
		query := Query{Id: fmt.Sprint(i), SessionId: util.GetContextSessionId(ctx), Type: QueryTypeInput, TriggerKeyword: "silent", RawQuery: "silent"}
		require.True(t, manager.QuerySilent(ctx, query))
		require.EqualValues(t, i+1, plugin.released.Load())
		_, retained := manager.resultStore.sessions.Load(query.SessionId)
		require.False(t, retained, "temporary hotkey session leaked")
	}
	require.EqualValues(t, 3, plugin.actions.Load())
	_, retained := manager.resultStore.getQuery("visible", "keep")
	require.True(t, retained, "unrelated window session was released")
}
