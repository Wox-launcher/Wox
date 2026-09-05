//go:build wox_ui_smoke

package clipboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/ui/automation"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestClipboardResultWaitsForCompletion reproduces input arriving before async results.
func TestClipboardResultWaitsForCompletion(t *testing.T) {
	const marker = "clipboard-wait-regression"
	var mu sync.Mutex
	query, polls, queries := "", 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var request struct {
			Method string
			Params struct{ Value string }
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		var result any
		switch request.Method {
		case "semantics.perform":
			query, polls = request.Params.Value, 0
			queries++
		case "semantics.snapshot":
			polls++
			nodes := []woxui.AccessibilityNode{
				{AutomationID: "launcher.query.input", Value: query},
				{AutomationID: "launcher.results", Value: "loading"},
			}
			if polls >= 3 {
				nodes[1].Value = "complete"
				nodes = append(nodes, woxui.AccessibilityNode{AutomationID: "launcher.result.marker", Label: marker})
			}
			result = woxwidget.AutomationSnapshot{Tree: woxui.AccessibilityTree{Nodes: nodes}}
		default:
			t.Errorf("unexpected method %q", request.Method)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"result": result}); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()
	client, err := automationdriver.NewClient(automation.Info{Address: server.URL, Token: "test-token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	waitForClipboardResult(t, ctx, client, marker)
	mu.Lock()
	defer mu.Unlock()
	if queries != 1 {
		t.Fatalf("submitted %d queries, want one completed query", queries)
	}
}
