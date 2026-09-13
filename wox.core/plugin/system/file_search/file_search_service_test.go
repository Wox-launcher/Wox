package system

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"wox/plugin"
	"wox/util/filesearch"
)

type serviceSyncTestAPI struct {
	fileSearchToolbarTestAPI
	done chan error
}

func (a serviceSyncTestAPI) Log(ctx context.Context, _ plugin.LogLevel, message string) {
	if strings.HasPrefix(message, "Failed to sync file search roots:") {
		a.done <- ctx.Err()
	}
}

// TestServiceActionReturnsBeforeBackgroundSync keeps scanner work out of startup feedback.
func TestServiceActionReturnsBeforeBackgroundSync(t *testing.T) {
	previous := executeFileIndexServiceFn
	defer func() { executeFileIndexServiceFn = previous }()
	executeFileIndexServiceFn = func(context.Context, string) error { return nil }
	api := serviceSyncTestAPI{done: make(chan error, 1)}
	c := &FileSearchPlugin{api: api, engine: &filesearch.Engine{}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.rootsSyncMu.Lock()
	returned := make(chan error, 1)
	go func() { returned <- c.HandleSettingAction(ctx, "start") }()
	select {
	case err := <-returned:
		if err != nil {
			c.rootsSyncMu.Unlock()
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		c.rootsSyncMu.Unlock()
		t.Fatal("service action waited for background synchronization")
	}
	cancel()
	c.rootsSyncMu.Unlock()
	select {
	case err := <-api.done:
		if err != nil {
			t.Fatalf("UI completion cancelled background synchronization: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("background synchronization did not run")
	}
	// A service failure must still reach the UI without scheduling reconciliation.
	want := errors.New("service start failed")
	executeFileIndexServiceFn = func(context.Context, string) error { return want }
	if err := c.HandleSettingAction(context.Background(), "start"); !errors.Is(err, want) {
		t.Fatalf("service error = %v, want %v", err, want)
	}
}
