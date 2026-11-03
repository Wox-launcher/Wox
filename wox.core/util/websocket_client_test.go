package util

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebsocketClientCloseWhileConnecting(t *testing.T) {
	requestReceived := make(chan struct{})
	continueHandshake := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		close(requestReceived)
		<-continueHandshake
		conn, err := (&websocket.Upgrader{}).Upgrade(writer, request, nil)
		if err == nil {
			defer conn.Close()
			_, _, _ = conn.ReadMessage()
		}
	}))
	defer server.Close()

	client := NewWebsocketClient("ws" + strings.TrimPrefix(server.URL, "http"))
	connectResult := make(chan error, 1)
	go func() {
		connectResult <- client.Connect(context.Background())
	}()

	<-requestReceived
	client.Close(context.Background())
	close(continueHandshake)

	require.Error(t, <-connectResult)
	assert.False(t, client.IsConnected())
}

func TestWebsocketClientHandshakeHeadersSurviveReconnect(t *testing.T) {
	received := make(chan string, 2)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received <- request.Header.Get("X-Wox-Host-Secret")
		conn, err := (&websocket.Upgrader{}).Upgrade(writer, request, http.Header{"X-Wox-Host-Protocol": {"1"}})
		if err == nil {
			defer conn.Close()
			_, _, _ = conn.ReadMessage()
		}
	}))
	defer server.Close()

	headers := http.Header{"X-Wox-Host-Secret": {"original"}}
	client := NewWebsocketClientWithHandshake("ws"+strings.TrimPrefix(server.URL, "http"), headers, http.Header{"X-Wox-Host-Protocol": {"1"}})
	defer client.Close(context.Background())
	headers.Set("X-Wox-Host-Secret", "changed")
	for range 2 {
		require.NoError(t, client.Connect(context.Background()))
		assert.Equal(t, "original", <-received)
	}
}

func TestWebsocketClientRejectsOutdatedHandshake(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(writer, request, nil)
		if err == nil {
			defer conn.Close()
			_, _, _ = conn.ReadMessage()
		}
	}))
	defer server.Close()
	client := NewWebsocketClientWithHandshake("ws"+strings.TrimPrefix(server.URL, "http"), nil, http.Header{"X-Wox-Host-Protocol": {"1"}})
	defer client.Close(context.Background())
	require.ErrorContains(t, client.Connect(context.Background()), "X-Wox-Host-Protocol")
	assert.False(t, client.IsConnected())
}
