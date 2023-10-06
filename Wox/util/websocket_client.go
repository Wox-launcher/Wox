package util

import (
	"context"
	"fmt"
	"github.com/gorilla/websocket"
	"sync"
	"time"
)

type WebsocketClient struct {
	url                  string
	conn                 *websocket.Conn
	cancelReceiveMsgChan chan bool
	onReceiveMsg         func(data []byte)
	reconnectCount       int
	isConnected          bool
	mu                   sync.RWMutex
}

func NewWebsocketClient(url string) *WebsocketClient {
	return &WebsocketClient{url: url}
}

func (w *WebsocketClient) Connect(ctx context.Context) error {
	w.disconnect(ctx)

	conn, _, dialErr := websocket.DefaultDialer.Dial(w.url, nil)
	if dialErr != nil {
		return dialErr
	}
	w.conn = conn
	w.cancelReceiveMsgChan = make(chan bool)
	w.isConnected = true

	Go(ctx, "receive websocket msg", func() {
		w.receiveMsg(ctx)
	})

	Go(ctx, "ping websocket server", func() {
		w.ping(ctx)
	})

	return nil
}

func (w *WebsocketClient) ping(ctx context.Context) {
	for {
		select {
		case <-time.NewTicker(time.Second).C:
			if w.conn != nil && w.isConnected {
				w.sendMsg(ctx, websocket.PingMessage, []byte{})
			}
		case <-w.cancelReceiveMsgChan:
			GetLogger().Info(ctx, "disconnect signal received, stop pinging")
			return
		}
	}
}

func (w *WebsocketClient) receiveMsg(ctx context.Context) {
	for {
		select {
		case <-w.cancelReceiveMsgChan:
			GetLogger().Info(ctx, "disconnect signal received, stop receiving message")
			return
		default:
			messageType, messageData, err := w.conn.ReadMessage()
			if err != nil {
				w.reconnect(ctx, fmt.Sprintf("failed to read message from websocket server (%s)", err.Error()))
				return
			}

			if messageType == websocket.TextMessage {
				if w.onReceiveMsg != nil {
					w.onReceiveMsg(messageData)
				}
			}
		}
	}
}

func (w *WebsocketClient) reconnect(ctx context.Context, reason string) {
	GetLogger().Info(ctx, fmt.Sprintf("%s, try reconnecting", reason))
	connErr := w.Connect(ctx)
	if connErr != nil {
		GetLogger().Error(ctx, fmt.Sprintf("connect websocket failed: %s", connErr))
		if w.reconnectCount > 10 {
			w.reconnectCount = w.reconnectCount * 2
		} else {
			w.reconnectCount++
		}
		GetLogger().Error(ctx, fmt.Sprintf("try to reconnect in %ds", w.reconnectCount))
		time.Sleep(time.Second * time.Duration(w.reconnectCount))
		w.reconnect(ctx, "failed to reconnect websocket")
	} else {
		GetLogger().Info(ctx, "reconnected websocket")
		w.reconnectCount = 0
	}
}

func (w *WebsocketClient) close(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil {
		w.conn.Close()
		w.conn = nil
	}
}

func (w *WebsocketClient) Send(ctx context.Context, data []byte) error {
	return w.sendMsg(ctx, websocket.TextMessage, data)
}

func (w *WebsocketClient) sendMsg(ctx context.Context, msgType int, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.conn.WriteMessage(msgType, data)
}

func (w *WebsocketClient) disconnect(ctx context.Context) {
	if w.cancelReceiveMsgChan == nil && w.conn == nil && !w.isConnected {
		return
	}

	GetLogger().Info(ctx, "disconnecting existing websocket client")

	if w.cancelReceiveMsgChan != nil {
		select {
		case w.cancelReceiveMsgChan <- true:
			close(w.cancelReceiveMsgChan)
		default:
			close(w.cancelReceiveMsgChan)
		}
		w.cancelReceiveMsgChan = nil
	}

	w.close(ctx)

	w.isConnected = false
}

func (w *WebsocketClient) OnMessage(ctx context.Context, callback func(data []byte)) {
	w.onReceiveMsg = callback
}
