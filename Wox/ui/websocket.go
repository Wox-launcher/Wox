package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/olahol/melody"
	"net/http"
	"os"
	"strings"
	"wox/plugin"
	"wox/resource"
	"wox/util"
)

var m *melody.Melody

type websocketMsgType string

const (
	WebsocketMsgTypeRequest  websocketMsgType = "WebsocketMsgTypeRequest"
	WebsocketMsgTypeResponse websocketMsgType = "WebsocketMsgTypeResponse"
)

type WebsocketMsg struct {
	Id      string
	Type    websocketMsgType
	Method  string
	Success bool
	Data    any
}

func serveAndWait(ctx context.Context, port int) {
	m = melody.New()
	m.Config.MaxMessageSize = 1024 * 1024 * 10 // 10MB
	m.Config.MessageBufferSize = 1024 * 1024   // 1MB

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Wox"))
	})

	http.HandleFunc("/image", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			w.Write([]byte("no id"))
			return
		}

		imagePath, ok := plugin.GetLocalImageMap(id)
		if !ok {
			w.Write([]byte("no image"))
			return
		}

		if _, statErr := os.Stat(imagePath); os.IsNotExist(statErr) {
			w.Write([]byte("image not exist"))
			return
		}

		w.Header().Set("Cache-Control", "public, max-age=3600")
		http.ServeFile(w, r, imagePath)
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		m.HandleRequest(w, r)
	})

	http.HandleFunc("/theme", func(w http.ResponseWriter, r *http.Request) {
		defaultTheme, defaultErr := resource.GetUITheme(ctx, "default")
		if defaultErr != nil {
			w.Write([]byte(defaultErr.Error()))
			return
		}

		w.Write(defaultTheme)
	})

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		ctxNew := util.NewTraceContext()

		logger.Info(ctxNew, fmt.Sprintf("got request from ui: %s", string(msg)))

		if strings.Contains(string(msg), string(WebsocketMsgTypeRequest)) {
			var request WebsocketMsg
			unmarshalErr := json.Unmarshal(msg, &request)
			if unmarshalErr != nil {
				logger.Error(ctxNew, fmt.Sprintf("failed to unmarshal websocket request: %s", unmarshalErr.Error()))
				return
			}
			util.Go(ctxNew, "handle ui query", func() {
				onUIRequest(ctxNew, request)
			})
		}
	})

	logger.Info(ctx, fmt.Sprintf("websocket server start at：ws://localhost:%d", port))
	err := http.ListenAndServe(fmt.Sprintf("localhost:%d", port), nil)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to start server: %s", err.Error()))
	}
}

func requestUI(ctx context.Context, request WebsocketMsg) {
	request.Type = WebsocketMsgTypeRequest
	marshalData, marshalErr := json.Marshal(request)
	if marshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to marshal websocket request: %s", marshalErr.Error()))
		return
	}

	jsonData, _ := json.Marshal(request.Data)
	util.GetLogger().Info(ctx, fmt.Sprintf("[->UI] %s: %s", request.Method, jsonData))
	m.Broadcast(marshalData)
}

func responseUI(ctx context.Context, response WebsocketMsg) {
	response.Type = WebsocketMsgTypeResponse
	marshalData, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to marshal websocket response: %s", marshalErr.Error()))
		return
	}
	m.Broadcast(marshalData)
}

func responseUISuccessWithData(ctx context.Context, request WebsocketMsg, data any) {
	responseUI(ctx, WebsocketMsg{
		Id:      request.Id,
		Type:    WebsocketMsgTypeResponse,
		Method:  request.Method,
		Success: true,
		Data:    data,
	})
}

func responseUISuccess(ctx context.Context, request WebsocketMsg) {
	responseUISuccessWithData(ctx, request, nil)
}

func responseUIError(ctx context.Context, request WebsocketMsg, errMsg string) {
	responseUI(ctx, WebsocketMsg{
		Id:      request.Id,
		Type:    WebsocketMsgTypeResponse,
		Method:  request.Method,
		Success: false,
		Data:    errMsg,
	})
}
