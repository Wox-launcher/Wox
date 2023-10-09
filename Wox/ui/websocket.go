package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/olahol/melody"
	"net/http"
	"time"
	"wox/plugin"
	"wox/util"
)

var m *melody.Melody

type websocketRequest struct {
	Id     string
	Method string
	Params map[string]string
}

type websocketResponse struct {
	Id     string
	Method string
	Data   any
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
			switch request.Method {
			case "query":
				handleQuery(ctxNew, request)
			case "action":
				handleAction(ctxNew, request)
			case "registerMainHotkey":
				handleRegisterMainHotkey(ctxNew, request)
			}
		})
	})

	logger.Info(ctx, fmt.Sprintf("websocket server start at：ws://localhost:%d", port))
	err := http.ListenAndServe(fmt.Sprintf("localhost:%d", port), nil)
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("failed to start server: %s", err.Error()))
	}
}

func handleQuery(ctx context.Context, request websocketRequest) {
	query, ok := request.Params["query"]
	if !ok {
		logger.Error(ctx, "query parameter not found")
		return
	}

	resultChan, doneChan := plugin.GetPluginManager().Query(ctx, plugin.NewQuery(query))
	select {
	case results := <-resultChan:
		logger.Info(ctx, fmt.Sprintf("query result count: %d", len(results)))
		if len(results) == 0 {
			return
		}

		response := websocketResponse{
			Id:     request.Id,
			Method: request.Method,
			Data:   plugin.NewQueryResultForUIs(results),
		}

		marshalData, marshalErr := json.Marshal(response)
		if marshalErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to marshal websocket response: %s", marshalErr.Error()))
			return
		}

		m.Broadcast(marshalData)
	case <-doneChan:
		logger.Info(ctx, "query done")
	case <-time.After(time.Second * 30):
		logger.Info(ctx, fmt.Sprintf("query timeout, query: %s, request id: %s", query, request.Id))
	}
}

func handleAction(ctx context.Context, request websocketRequest) {
	resultId, ok := request.Params["id"]
	if !ok {
		logger.Error(ctx, "id parameter not found")
		return
	}

	action := plugin.GetActionForResult(resultId)
	if action == nil {
		logger.Error(ctx, fmt.Sprintf("action not found for result id: %s", resultId))
		return
	}

	action()
}

func handleRegisterMainHotkey(ctx context.Context, request websocketRequest) {
	hotkey, ok := request.Params["hotkey"]
	if !ok {
		logger.Error(ctx, "hotkey parameter not found")
		return
	}

	registerErr := GetUIManager().RegisterMainHotkey(ctx, hotkey)
	if registerErr != nil {
		responseUI(ctx, websocketResponse{
			Id:     request.Id,
			Method: request.Method,
			Data:   registerErr.Error(),
		})
	} else {
		responseUI(ctx, websocketResponse{
			Id:     request.Id,
			Method: request.Method,
			Data:   "success",
		})
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
