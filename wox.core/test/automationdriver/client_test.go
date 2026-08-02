package automationdriver

import (
	"context"
	"encoding/json"
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
