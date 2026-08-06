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
