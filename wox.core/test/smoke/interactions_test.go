package smoke

import (
	"context"
	"encoding/json"
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

// TestOpenInstalledPluginSettingsWaitsForSelectedFrame keeps the previous plugin's controls from satisfying later waits.
func TestOpenInstalledPluginSettingsWaitsForSelectedFrame(t *testing.T) {
	t.Setenv(automationdriver.SmokeStepDelayEnvironment, "0")
	const pluginID = "calculator"
	var selectionRequested atomic.Bool
	var snapshotsAfterSelection atomic.Int32
	var selectedFramePublished atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     uint64
			Method string
			Params struct {
				AutomationID string `json:"automationId"`
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode automation request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var result any = true
		switch request.Method {
		case "window.open_settings":
		case "semantics.perform":
			if request.Params.AutomationID == "plugin-list-"+pluginID {
				selectionRequested.Store(true)
			}
		case "semantics.snapshot":
			// Selection is accepted immediately, while frames still expose the
			// previously selected plugin and its generic enabled Disable button.
			selected := selectionRequested.Load() && snapshotsAfterSelection.Add(1) >= 3
			selectedFramePublished.Store(selected)
			result = woxwidget.AutomationSnapshot{Tree: woxui.AccessibilityTree{Nodes: []woxui.AccessibilityNode{
				{AutomationID: "plugin-search", Enabled: true},
				{AutomationID: "plugin-list-" + pluginID, Enabled: true, Selected: selected},
				{AutomationID: "plugin-disable", Enabled: true},
			}}}
		default:
			t.Errorf("unexpected automation method %q", request.Method)
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}); err != nil {
			t.Errorf("encode automation response: %v", err)
		}
	}))
	defer server.Close()
	client, err := automationdriver.NewClient(automation.Info{Address: server.URL, Token: "test-token"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	OpenInstalledPluginSettings(t, ctx, client, pluginID)

	if !selectedFramePublished.Load() {
		t.Fatal("returned before the requested plugin was selected in the semantics tree")
	}
}
