package automationdriver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"wox/ui/automation"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestNewClientReadsSmokeStepDelay(t *testing.T) {
	t.Setenv(SmokeStepDelayEnvironment, "250ms")
	client, err := NewClient(automation.Info{Address: "http://wox-automation.test", Token: "test-token"})
	if err != nil {
		t.Fatalf("create slow client: %v", err)
	}
	if client.stepDelay != 250*time.Millisecond {
		t.Fatalf("step delay = %s, want 250ms", client.stepDelay)
	}
}

func TestClientAuthenticatesAndDecodesSnapshot(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("missing authentication header")
		}
		var requestPayload struct {
			ID     uint64 `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(request.Body).Decode(&requestPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if requestPayload.Method != "semantics.snapshot" {
			t.Fatalf("unexpected method %q", requestPayload.Method)
		}
		body, err := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      requestPayload.ID,
			"result": map[string]any{
				"Tree": map[string]any{
					"Generation": 7,
					"RootIDs":    []int{1},
					"Nodes": []map[string]any{{
						"ID":           1,
						"AutomationID": "launcher.query",
						"Role":         "text_field",
					}},
				},
			},
		})
		if err != nil {
			t.Fatalf("encode response: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Header:     make(http.Header),
		}, nil
	})

	client, err := NewClient(automation.Info{Address: "http://wox-automation.test", Token: "test-token"})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	client.http.Transport = transport
	snapshot, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if snapshot.Tree.Generation != 7 {
		t.Fatalf("expected generation 7, got %d", snapshot.Tree.Generation)
	}
	node, found := Find(snapshot, "launcher.query")
	if !found || node.Role != woxui.AccessibilityRoleTextField {
		t.Fatalf("unexpected query node: found=%v node=%+v", found, node)
	}
}

func TestWaitForReturnsLastSnapshotWhenWaitingFails(t *testing.T) {
	t.Parallel()

	requestCount := 0
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestCount++
		if requestCount > 1 {
			return nil, errors.New("wait transport failed")
		}
		body, err := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{"Tree": map[string]any{
				"Generation": 9,
				"Nodes":      []map[string]any{{"ID": 1, "AutomationID": "launcher.query.input", "Value": "last value"}},
			}},
		})
		if err != nil {
			t.Fatalf("encode snapshot response: %v", err)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})

	client, err := NewClient(automation.Info{Address: "http://wox-automation.test", Token: "test-token"})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	client.http.Transport = transport
	snapshot, err := client.WaitFor(context.Background(), func(woxwidget.AutomationSnapshot) bool { return false })
	if err == nil || !strings.Contains(err.Error(), "after generation 9") {
		t.Fatalf("wait error = %v, want generation context", err)
	}
	node, found := Find(snapshot, "launcher.query.input")
	if !found || node.Value != "last value" {
		t.Fatalf("last snapshot was not preserved: found=%v node=%+v", found, node)
	}
}

func TestPressKeyHandledReturnsServerResult(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var requestPayload struct {
			ID     uint64 `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(request.Body).Decode(&requestPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if requestPayload.Method != "input.key" {
			t.Fatalf("unexpected method %q", requestPayload.Method)
		}
		body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": requestPayload.ID, "result": true})
		if err != nil {
			t.Fatalf("encode response: %v", err)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})

	client, err := NewClient(automation.Info{Address: "http://wox-automation.test", Token: "test-token"})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	client.http.Transport = transport
	handled, err := client.PressKeyHandled(context.Background(), woxui.Key("v"), woxui.KeyModifierMeta)
	if err != nil || !handled {
		t.Fatalf("press handled = %v err %v, want true", handled, err)
	}
}

func TestSendKeyHandledDispatchesKeyEvent(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var requestPayload struct {
			ID     uint64 `json:"id"`
			Method string `json:"method"`
			Params struct {
				Key       woxui.Key          `json:"key"`
				Modifiers woxui.KeyModifiers `json:"modifiers"`
				Down      bool               `json:"down"`
			} `json:"params"`
		}
		if err := json.NewDecoder(request.Body).Decode(&requestPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if requestPayload.Method != "input.key_event" || requestPayload.Params.Key != woxui.KeyAlt || requestPayload.Params.Modifiers != woxui.KeyModifierAlt || !requestPayload.Params.Down {
			t.Fatalf("unexpected key event %#v", requestPayload)
		}
		body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": requestPayload.ID, "result": true})
		if err != nil {
			t.Fatalf("encode response: %v", err)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})

	client, err := NewClient(automation.Info{Address: "http://wox-automation.test", Token: "test-token"})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	client.http.Transport = transport
	handled, err := client.SendKeyHandled(context.Background(), woxui.KeyAlt, woxui.KeyModifierAlt, true)
	if err != nil || !handled {
		t.Fatalf("send handled = %v err %v, want true", handled, err)
	}
}

func TestClientReadsAndResetsFrameMetrics(t *testing.T) {
	t.Parallel()

	var methods []string
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var requestPayload struct {
			ID     uint64 `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(request.Body).Decode(&requestPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		methods = append(methods, requestPayload.Method)
		result := any(true)
		if requestPayload.Method == "render.metrics" {
			result = map[string]any{"frameCount": 12, "presentedFrameCount": 11}
		}
		body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": requestPayload.ID, "result": result})
		if err != nil {
			t.Fatalf("encode response: %v", err)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})

	client, err := NewClient(automation.Info{Address: "http://wox-automation.test", Token: "test-token"})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	client.http.Transport = transport
	metrics, err := client.FrameMetrics(context.Background())
	if err != nil {
		t.Fatalf("read frame metrics: %v", err)
	}
	if metrics.FrameCount != 12 || metrics.PresentedFrameCount != 11 {
		t.Fatalf("unexpected frame metrics: %+v", metrics)
	}
	if err := client.ResetFrameMetrics(context.Background()); err != nil {
		t.Fatalf("reset frame metrics: %v", err)
	}
	if err := client.RequestFrame(context.Background()); err != nil {
		t.Fatalf("request frame: %v", err)
	}
	if err := client.SetRepaintDebugMode(context.Background(), woxwidget.RepaintDebugVerify); err != nil {
		t.Fatalf("set repaint debug mode: %v", err)
	}
	if err := client.SimulateRendererDeviceRemoved(context.Background()); err != nil {
		t.Fatalf("simulate renderer device removal: %v", err)
	}
	if len(methods) != 5 || methods[0] != "render.metrics" || methods[1] != "render.metrics.reset" || methods[2] != "render.invalidate" || methods[3] != "render.repaint_debug" || methods[4] != "render.simulate_device_removed" {
		t.Fatalf("unexpected methods: %v", methods)
	}
}

func TestClientOpensSettingsRoute(t *testing.T) {
	t.Parallel()

	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var requestPayload struct {
			ID     uint64 `json:"id"`
			Method string `json:"method"`
			Params struct {
				Path string `json:"path"`
			} `json:"params"`
		}
		if err := json.NewDecoder(request.Body).Decode(&requestPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if requestPayload.Method != "window.open_settings" {
			t.Fatalf("unexpected method %q", requestPayload.Method)
		}
		if requestPayload.Params.Path != "/appearance" {
			t.Fatalf("unexpected settings path %q", requestPayload.Params.Path)
		}
		body, err := json.Marshal(map[string]any{
			"jsonrpc": "2.0",
			"id":      requestPayload.ID,
			"result":  true,
		})
		if err != nil {
			t.Fatalf("encode response: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Header:     make(http.Header),
		}, nil
	})

	client, err := NewClient(automation.Info{Address: "http://wox-automation.test", Token: "test-token"})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	client.http.Transport = transport
	if err := client.OpenSettings(context.Background(), "/appearance"); err != nil {
		t.Fatalf("open settings route: %v", err)
	}
}

func TestClientOpensSelectionQueryAndReadsWindowState(t *testing.T) {
	t.Parallel()

	var methods []string
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var requestPayload struct {
			ID     uint64 `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(request.Body).Decode(&requestPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		methods = append(methods, requestPayload.Method)
		result := any(true)
		if requestPayload.Method == "window.state" {
			result = automation.WindowState{Exists: true, Visible: true, BlurReady: true, Lifecycle: "visible"}
		}
		body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": requestPayload.ID, "result": result})
		if err != nil {
			t.Fatalf("encode response: %v", err)
		}
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})

	client, err := NewClient(automation.Info{Address: "http://wox-automation.test", Token: "test-token"})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	client.http.Transport = transport
	if err := client.OpenSelectionQuery(context.Background(), "selected text"); err != nil {
		t.Fatalf("open selection query: %v", err)
	}
	state, err := client.WindowState(context.Background(), "selection")
	if err != nil {
		t.Fatalf("read selection window state: %v", err)
	}
	if !state.Exists || !state.Visible || !state.BlurReady || state.Lifecycle != "visible" {
		t.Fatalf("unexpected selection window state: %+v", state)
	}
	if len(methods) != 2 || methods[0] != "window.open_selection_query" || methods[1] != "window.state" {
		t.Fatalf("unexpected methods: %v", methods)
	}
}

func TestClientMovesPointerToSemanticsNodeCenter(t *testing.T) {
	t.Parallel()

	var methods []string
	var pointer woxui.PointerEvent
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var requestPayload struct {
			ID     uint64          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(request.Body).Decode(&requestPayload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		methods = append(methods, requestPayload.Method)
		result := any(true)
		if requestPayload.Method == "semantics.snapshot" {
			result = map[string]any{"Tree": map[string]any{
				"Generation": 1,
				"Nodes": []map[string]any{{
					"ID": 1, "AutomationID": "plan-info", "Role": "image",
					"Bounds": map[string]any{"X": 100, "Y": 40, "Width": 14, "Height": 14},
				}},
			}}
		} else if requestPayload.Method == "input.pointer" {
			if err := json.Unmarshal(requestPayload.Params, &pointer); err != nil {
				t.Fatalf("decode pointer: %v", err)
			}
		}
		body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": requestPayload.ID, "result": result})
		if err != nil {
			t.Fatalf("encode response: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Header:     make(http.Header),
		}, nil
	})

	client, err := NewClient(automation.Info{Address: "http://wox-automation.test", Token: "test-token"})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	client.http.Transport = transport
	node, err := client.MovePointerTo(context.Background(), "plan-info")
	if err != nil {
		t.Fatalf("move pointer: %v", err)
	}
	if node.AutomationID != "plan-info" {
		t.Fatalf("unexpected node: %+v", node)
	}
	if len(methods) != 2 || methods[0] != "semantics.snapshot" || methods[1] != "input.pointer" {
		t.Fatalf("unexpected methods: %v", methods)
	}
	if pointer.Kind != woxui.PointerMove || pointer.Position != (woxui.Point{X: 107, Y: 47}) {
		t.Fatalf("unexpected pointer event: %+v", pointer)
	}
}

func TestFindByAutomationIDPrefix(t *testing.T) {
	snapshot := woxwidget.AutomationSnapshot{Tree: woxui.AccessibilityTree{Nodes: []woxui.AccessibilityNode{
		{AutomationID: "terminal-search-input-first"},
		{AutomationID: "terminal-search-next-first"},
	}}}
	node, found := FindByAutomationIDPrefix(snapshot, "terminal-search-next-")
	if !found || node.AutomationID != "terminal-search-next-first" {
		t.Fatalf("dynamic node = found %v, id %q", found, node.AutomationID)
	}
}

type roundTripFunc func(request *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
