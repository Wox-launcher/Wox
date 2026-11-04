package plugin

import (
	"context"
	"testing"
	"wox/common"
	"wox/util"
)

type removeResultTestUI struct {
	common.UI
	accepted bool
}

func (u *removeResultTestUI) RemoveResult(_ context.Context, _ interface{}) bool { return u.accepted }

func TestRemoveResultOwnershipScopeAndCache(t *testing.T) {
	previous := GetPluginManager()
	defer func() { managerInstance = previous }()
	query := Query{Id: "query", SessionId: "session"}
	manager, owner := newTestManagerWithCachedResult(query, QueryResult{Id: "row"})
	ui := &removeResultTestUI{}
	manager.ui = ui
	managerInstance = manager
	ctx := util.WithQueryIdContext(util.WithSessionContext(context.Background(), "session"), "query")
	option := RemoveResultOption{ResultId: "row"}
	foreign := &APIImpl{pluginInstance: &Instance{}}
	if foreign.RemoveResult(ctx, option).Success {
		t.Fatal("foreign owner accepted")
	}
	api := &APIImpl{pluginInstance: owner}
	if api.RemoveResult(util.WithQueryIdContext(ctx, "old"), option).Success {
		t.Fatal("stale scope accepted")
	}
	if api.RemoveResult(util.WithSessionContext(context.Background(), "other"), option).Success {
		t.Fatal("other session resolved an unscoped result")
	}
	if api.RemoveResult(util.WithQueryIdContext(context.Background(), "query"), option).Success {
		t.Fatal("query without session accepted")
	}
	if api.RemoveResult(ctx, option).Success {
		t.Fatal("UI rejection accepted")
	}
	if _, ok := manager.resultStore.findWithContext(ctx, "row"); !ok {
		t.Fatal("rejected result removed from cache")
	}
	ui.accepted = true
	if !api.RemoveResult(ctx, option).Success {
		t.Fatal("removal rejected")
	}
	if _, ok := manager.resultStore.findWithContext(ctx, "row"); ok {
		t.Fatal("removed result retained in cache")
	}
	if api.RemoveResult(ctx, option).Success {
		t.Fatal("duplicate removal accepted")
	}
}
