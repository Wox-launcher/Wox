package util

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type WebsocketClient struct {
	url                  string
	conn                 *websocket.Conn
	cancelReceiveMsgChan chan struct{}
	onReceiveMsg         func(data []byte)
	reconnectCount       int
	isConnected          bool
	mu                   sync.RWMutex
	// stopped is closed by Close to make a pending reconnect loop give up.
	stopped  chan struct{}
	stopOnce sync.Once
}

func NewWebsocketClient(url string) *WebsocketClient {
	return &WebsocketClient{url: url, stopped: make(chan struct{})}
}

func (w *WebsocketClient) Connect(ctx context.Context) error {
	w.disconnect(ctx)

	select {
	case <-w.stopped:
		GetLogger().Info(ctx, "websocket client closed, skip connect")
		return fmt.Errorf("websocket client is closed")
	default:
	}

	conn, _, dialErr := websocket.DefaultDialer.Dial(w.url, nil)
	if dialErr != nil {
		return dialErr
	}

	cancelReceiveMsgChan := make(chan struct{})
	w.mu.Lock()
	select {
	case <-w.stopped:
		w.mu.Unlock()
		conn.Close()
		return fmt.Errorf("websocket client is closed")
	default:
	}
	w.conn = conn
	w.cancelReceiveMsgChan = cancelReceiveMsgChan
	w.isConnected = true
	w.mu.Unlock()

	Go(ctx, "receive websocket msg", func() {
		w.receiveMsg(ctx, conn, cancelReceiveMsgChan)
	})

	Go(ctx, "ping websocket server", func() {
		w.ping(ctx, conn, cancelReceiveMsgChan)
	})

	return nil
}

func (w *WebsocketClient) IsConnected() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.isConnected
}

func (w *WebsocketClient) ping(ctx context.Context, conn *websocket.Conn, cancelReceiveMsgChan <-chan struct{}) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.sendMsgToConn(ctx, conn, websocket.PingMessage, []byte{})
		case <-cancelReceiveMsgChan:
			GetLogger().Info(ctx, "disconnect signal received, stop pinging")
			return
		case <-w.stopped:
			GetLogger().Info(ctx, "websocket client closed, stop pinging")
			return
		}
	}
}

func (w *WebsocketClient) receiveMsg(ctx context.Context, conn *websocket.Conn, cancelReceiveMsgChan <-chan struct{}) {
	for {
		select {
		case <-cancelReceiveMsgChan:
			GetLogger().Info(ctx, "disconnect signal received, stop receiving message")
			return
		default:
			messageType, messageData, err := conn.ReadMessage()
			if err != nil {
				select {
				case <-cancelReceiveMsgChan:
					return
				case <-w.stopped:
					return
				default:
				}
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

// Close permanently shuts down the client: the receive goroutine exits and any
// pending reconnect loop gives up instead of retrying the old address forever.
// It is used when the underlying host process is stopped or replaced.
func (w *WebsocketClient) Close(ctx context.Context) {
	w.stopOnce.Do(func() {
		close(w.stopped)
	})
	w.disconnect(ctx)
}

func (w *WebsocketClient) reconnect(ctx context.Context, reason string) {
	for {
		GetLogger().Info(ctx, fmt.Sprintf("%s, try reconnecting", reason))
		connErr := w.Connect(ctx)
		if connErr == nil {
			GetLogger().Info(ctx, "reconnected websocket")
			w.reconnectCount = 0
			return
		}

		GetLogger().Error(ctx, fmt.Sprintf("connect websocket failed: %s", connErr))
		if w.reconnectCount > 10 {
			w.reconnectCount = w.reconnectCount * 2
		} else {
			w.reconnectCount++
		}
		GetLogger().Error(ctx, fmt.Sprintf("try to reconnect in %ds", w.reconnectCount))
		select {
		case <-time.After(time.Second * time.Duration(w.reconnectCount)):
		case <-w.stopped:
			GetLogger().Info(ctx, "websocket client closed, stop reconnecting")
			return
		}
	}
}

func (w *WebsocketClient) Send(ctx context.Context, data []byte) error {
	return w.sendMsg(ctx, websocket.TextMessage, data)
}

func (w *WebsocketClient) sendMsg(ctx context.Context, msgType int, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn == nil {
		return fmt.Errorf("websocket client is not connected")
	}
	return w.conn.WriteMessage(msgType, data)
}

// sendMsgToConn prevents a stale ping loop from writing to a replacement connection.
func (w *WebsocketClient) sendMsgToConn(ctx context.Context, conn *websocket.Conn, msgType int, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != conn {
		return fmt.Errorf("websocket connection is no longer active")
	}
	return conn.WriteMessage(msgType, data)
}

func (w *WebsocketClient) disconnect(ctx context.Context) {
	w.mu.Lock()
	if w.cancelReceiveMsgChan == nil && w.conn == nil && !w.isConnected {
		w.mu.Unlock()
		return
	}

	GetLogger().Info(ctx, "disconnecting existing websocket client")
	cancelReceiveMsgChan := w.cancelReceiveMsgChan
	conn := w.conn
	w.cancelReceiveMsgChan = nil
	w.conn = nil
	w.isConnected = false
	w.mu.Unlock()

	if cancelReceiveMsgChan != nil {
		close(cancelReceiveMsgChan)
	}
	if conn != nil {
		conn.Close()
	}
}

func (w *WebsocketClient) OnMessage(ctx context.Context, callback func(data []byte)) {
	w.onReceiveMsg = callback
}
