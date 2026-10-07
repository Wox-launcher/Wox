package host

import (
	"context"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"wox/network"
	"wox/util"
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
