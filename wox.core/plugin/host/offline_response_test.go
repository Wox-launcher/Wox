package host

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"wox/network"
	"wox/plugin"
	"wox/util"

	"github.com/gorilla/websocket"
)

// TestOfflineCallbackGetsError verifies rejection is delivered before host shutdown.
func TestOfflineCallbackGetsError(t *testing.T) {
	received := make(chan JsonRpcResponse, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var response JsonRpcResponse
		if conn.ReadJSON(&response) == nil {
			received <- response
		}
	}))
	defer server.Close()
	previous := network.IsOffline()
	network.Default.SetOffline(true)
	defer network.Default.SetOffline(previous)
	client := util.NewWebsocketClient("ws" + strings.TrimPrefix(server.URL, "http"))
	if err := client.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer client.Close(context.Background())
	host := &WebsocketHost{ws: client}
	host.handleRequestFromPlugin(context.Background(), JsonRpcRequest{Id: "offline-request", Method: "ignored"})
	select {
	case response := <-received:
		if response.Id != "offline-request" || response.Error != network.ErrOffline.Error() {
			t.Fatalf("unexpected response: %+v", response)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("offline request was dropped")
	}
}

// TestOfflineAllowsResultRelease keeps local cleanup available without allowing plugin execution.
func TestOfflineAllowsResultRelease(t *testing.T) {
	host, _, requests := newPluginRequestPeer(t)
	network.Default.SetOffline(true)
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	ctx = util.WithQueryIdContext(util.WithSessionContext(ctx, "silent-session"), "query")
	metadata := plugin.Metadata{Id: "owner", Name: "Test"}
	if _, err := host.invokeMethod(ctx, metadata, "query", nil); !errors.Is(err, network.ErrOffline) {
		t.Fatalf("ordinary query bypassed offline mode: %v", err)
	}
	if _, err := host.invokeMethod(ctx, metadata, "releaseQueryResults", nil); err != nil {
		t.Fatalf("offline cleanup failed: %v", err)
	}
	select {
	case request := <-requests:
		if request.Method != "releaseQueryResults" || request.SessionId != "silent-session" || request.QueryId != "query" {
			t.Fatalf("unexpected cleanup request: %+v", request)
		}
	case <-ctx.Done():
		t.Fatal("cleanup was not delivered")
	}
	if host.requestMap.Len() != 0 {
		t.Fatal("completed cleanup retained its RPC waiter")
	}
}
