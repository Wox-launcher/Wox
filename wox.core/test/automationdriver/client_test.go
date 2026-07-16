package automationdriver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"wox/ui/automation"
	woxui "wox/ui/runtime"
)

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

type roundTripFunc func(request *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
