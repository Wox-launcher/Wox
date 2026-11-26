package host

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"wox/plugin"
	"wox/util"

	"github.com/gorilla/websocket"
)

func TestHostQueryCancellationCarriesRequestAndGeneration(t *testing.T) {
	requests := make(chan JsonRpcRequest, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var request JsonRpcRequest
			if conn.ReadJSON(&request) != nil {
				return
			}
			requests <- request
		}
	}))
	t.Cleanup(server.Close)
	client := util.NewWebsocketClient("ws" + strings.TrimPrefix(server.URL, "http"))
	if err := client.Connect(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close(context.Background()) })
	host := &WebsocketHost{ws: client, host: requestTestRuntime{}, requestMap: util.NewHashMap[string, chan JsonRpcResponse]()}

	for _, scenario := range []string{"superseded", "deadline"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := util.WithQueryIdContext(util.WithSessionContext(t.Context(), "launcher"), scenario)
			wantErr := context.Canceled
			var cancel context.CancelFunc
			if scenario == "deadline" {
				ctx, cancel = context.WithTimeout(ctx, 300*time.Millisecond)
				wantErr = context.DeadlineExceeded
			} else {
				ctx, cancel = context.WithCancel(ctx)
			}
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := host.invokeMethod(ctx, plugin.Metadata{Id: "fixture"}, "query", nil)
				done <- err
			}()
			request := takeQueryRPC(t, requests)
			if request.Method != "query" {
				t.Fatalf("expected query RPC, got %+v", request)
			}
			if scenario == "superseded" {
				cancel()
			}
			cancelRequest := takeQueryRPC(t, requests)
			if cancelRequest.Method != "cancelQuery" || cancelRequest.Params["RequestId"] != request.Id || cancelRequest.QueryId != scenario || cancelRequest.SessionId != "launcher" || cancelRequest.PluginId != "fixture" {
				t.Fatalf("cancel notification does not identify the query: %+v", cancelRequest)
			}
			if err := <-done; !errors.Is(err, wantErr) {
				t.Fatalf("query returned %v, want %v", err, wantErr)
			}
			if _, ok := host.requestMap.Load(request.Id); ok {
				t.Fatal("canceled request is still waiting for a reply")
			}
		})
	}
}

func takeQueryRPC(t *testing.T, requests <-chan JsonRpcRequest) JsonRpcRequest {
	t.Helper()
	select {
	case request := <-requests:
		return request
	case <-time.After(3 * time.Second):
		t.Fatal("expected host RPC was not received")
		return JsonRpcRequest{}
	}
}
