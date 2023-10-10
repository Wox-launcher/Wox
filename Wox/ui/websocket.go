package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/olahol/melody"
	"net/http"
	"wox/util"
)

var m *melody.Melody

type websocketRequest struct {
	Id     string
	Method string
	Params map[string]string
}

type websocketResponse struct {
	Id      string
	Method  string
	Success bool
	Data    any
}

func serveAndWait(ctx context.Context, port int) {
	m = melody.New()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Wox"))
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		m.HandleRequest(w, r)
	})

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		ctxNew := util.NewTraceContext()
		logger.Info(ctxNew, fmt.Sprintf("got request from ui: %s", string(msg)))

		var request websocketRequest
		unmarshalErr := json.Unmarshal(msg, &request)
		if unmarshalErr != nil {
			logger.Error(ctxNew, fmt.Sprintf("failed to unmarshal websocket request: %s", unmarshalErr.Error()))
			return
		}

		util.Go(ctxNew, "handle ui query", func() {
			onUIRequest(ctxNew, request)
		})
	})

	logger.Info(ctx, fmt.Sprintf("websocket server start at：ws://localhost:%d", port))
	err := http.ListenAndServe(fmt.Sprintf("localhost:%d", port), nil)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to start server: %s", err.Error()))
	}
}

func requestUI(ctx context.Context, request websocketRequest) {
	marshalData, marshalErr := json.Marshal(request)
	if marshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to marshal websocket request: %s", marshalErr.Error()))
		return
	}
	m.Broadcast(marshalData)
}

func responseUI(ctx context.Context, response websocketResponse) {
	marshalData, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to marshal websocket response: %s", marshalErr.Error()))
		return
	}
	m.Broadcast(marshalData)
}

func responseUISuccessWithData(ctx context.Context, request websocketRequest, data any) {
	responseUI(ctx, websocketResponse{
		Id:      request.Id,
		Method:  request.Method,
		Success: true,
		Data:    data,
	})
}

func responseUISuccess(ctx context.Context, request websocketRequest) {
	responseUISuccessWithData(ctx, request, nil)
}

func responseUIError(ctx context.Context, request websocketRequest, errMsg string) {
	responseUI(ctx, websocketResponse{
		Id:      request.Id,
		Method:  request.Method,
		Success: false,
		Data:    errMsg,
	})
}
