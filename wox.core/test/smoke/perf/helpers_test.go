//go:build wox_ui_smoke

package perf

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/ui/automation"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestSnapshotQuietIgnoresRedraws waits through content changes but not repeated identical frames.
func TestSnapshotQuietIgnoresRedraws(t *testing.T) {
	var calls, lastChange atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := calls.Add(1)
		if count <= 8 {
			lastChange.Store(time.Now().UnixNano())
		}
		snapshot := woxwidget.AutomationSnapshot{Tree: woxui.AccessibilityTree{
			Generation: uint64(count),
			Nodes:      []woxui.AccessibilityNode{{AutomationID: "chat.messages", Label: fmt.Sprint(min(count, 8))}},
		}}
		if err := json.NewEncoder(w).Encode(map[string]any{"result": snapshot}); err != nil {
			t.Errorf("encode snapshot: %v", err)
		}
	}))
	defer server.Close()
	client, err := automationdriver.NewClient(automation.Info{Address: server.URL, Token: "test-token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	const quiet = 100 * time.Millisecond
	waitForSnapshotQuiet(t, ctx, client, quiet)
	if calls.Load() <= 8 || time.Since(time.Unix(0, lastChange.Load())) < quiet {
		t.Fatal("quiet wait returned before the final content settled")
	}
}
