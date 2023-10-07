package main

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
var mainHotkey = util.Hotkey{}

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

func ServeAndWait(ctx context.Context, port int) {
	m = melody.New()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Wox"))
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		m.HandleRequest(w, r)
	})

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		ctxNew := util.NewTraceContext()
		util.GetLogger().Error(ctxNew, fmt.Sprintf("got request from ui: %s", string(msg)))

		var request websocketRequest
		unmarshalErr := json.Unmarshal(msg, &request)
		if unmarshalErr != nil {
			util.GetLogger().Error(ctxNew, fmt.Sprintf("failed to unmarshal websocket request: %s", unmarshalErr.Error()))
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

	util.GetLogger().Info(ctx, fmt.Sprintf("websocket server start at：ws://localhost:%d", port))
	err := http.ListenAndServe(fmt.Sprintf("localhost:%d", port), nil)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to start server: %s", err.Error()))
	}
}

func handleQuery(ctx context.Context, request websocketRequest) {
	query, ok := request.Params["query"]
	if !ok {
		util.GetLogger().Error(ctx, "query parameter not found")
		return
	}

	resultChan, doneChan := plugin.GetPluginManager().Query(ctx, plugin.NewQuery(query))
	for {
		select {
		case results := <-resultChan:
			util.GetLogger().Info(ctx, fmt.Sprintf("query result count: %d", len(results)))
			if len(results) == 0 {
				continue
			}

			response := websocketResponse{
				Id:     request.Id,
				Method: request.Method,
				Data:   plugin.NewQueryResultForUIs(results),
			}

			marshalData, marshalErr := json.Marshal(response)
			if marshalErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("failed to marshal websocket response: %s", marshalErr.Error()))
				continue
			}

			m.Broadcast(marshalData)
		case <-doneChan:
			util.GetLogger().Info(ctx, "query done")
			return
		case <-time.After(time.Second * 30):
			util.GetLogger().Info(ctx, "query timeout")
			return
		}
	}
}

func handleAction(ctx context.Context, request websocketRequest) {
	resultId, ok := request.Params["id"]
	if !ok {
		util.GetLogger().Error(ctx, "id parameter not found")
		return
	}

	action := plugin.GetActionForResult(resultId)
	if action == nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("action not found for result id: %s", resultId))
		return
	}

	hideWox := action()

	response := websocketResponse{
		Id:     request.Id,
		Method: request.Method,
		Data:   hideWox,
	}
	marshalData, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to marshal websocket response: %s", marshalErr.Error()))
		return
	}
	m.Broadcast(marshalData)
}

func handleRegisterMainHotkey(ctx context.Context, request websocketRequest) {
	hotkey, ok := request.Params["hotkey"]
	if !ok {
		util.GetLogger().Error(ctx, "hotkey parameter not found")
		return
	}

	mainHotkey.Register(ctx, hotkey, toggleWindow)
}

func RequestUI(ctx context.Context, request websocketRequest) {
	marshalData, marshalErr := json.Marshal(request)
	if marshalErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to marshal websocket request: %s", marshalErr.Error()))
		return
	}
	m.Broadcast(marshalData)
}
