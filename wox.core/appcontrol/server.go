package appcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"wox/util"
)

// Handlers contains the small set of cross-process controls exposed by the primary Wox instance.
type Handlers struct {
	Show                        func(ctx context.Context) error
	DeepLink                    func(ctx context.Context, deepLink string) error
	EnableDiagnosticsAndRestart func(ctx context.Context) (any, error)
}

type response struct {
	Success bool
	Message string
	Data    any
}

// NewHandler creates the loopback-only process-control API.
func NewHandler(handlers Handlers) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(writer http.ResponseWriter, request *http.Request) {
		writeSuccess(writer, "pong")
	})
	mux.HandleFunc("/show", func(writer http.ResponseWriter, request *http.Request) {
		if handlers.Show == nil {
			writeError(writer, "show handler is unavailable")
			return
		}
		if err := handlers.Show(traceContext(request)); err != nil {
			writeError(writer, err.Error())
			return
		}
		writeSuccess(writer, "")
	})
	mux.HandleFunc("/deeplink", func(writer http.ResponseWriter, request *http.Request) {
		if handlers.DeepLink == nil {
			writeError(writer, "deeplink handler is unavailable")
			return
		}
		var payload struct {
			DeepLink string `json:"deeplink"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			writeError(writer, "invalid deeplink request")
			return
		}
		payload.DeepLink = strings.TrimSpace(payload.DeepLink)
		if payload.DeepLink == "" {
			writeError(writer, "deeplink is empty")
			return
		}
		if err := handlers.DeepLink(traceContext(request), payload.DeepLink); err != nil {
			writeError(writer, err.Error())
			return
		}
		writeSuccess(writer, "")
	})
	mux.HandleFunc("/diagnostics/monitor/enable-restart", func(writer http.ResponseWriter, request *http.Request) {
		if handlers.EnableDiagnosticsAndRestart == nil {
			writeError(writer, "diagnostics handler is unavailable")
			return
		}
		state, err := handlers.EnableDiagnosticsAndRestart(traceContext(request))
		if err != nil {
			writeError(writer, err.Error())
			return
		}
		writeSuccess(writer, state)
	})
	return mux
}

// ServeAndWait runs the primary-instance control server until shutdown or failure.
func ServeAndWait(ctx context.Context, port int, handlers Handlers) error {
	server := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", port),
		Handler:           NewHandler(handlers),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	err := server.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func traceContext(request *http.Request) context.Context {
	traceID := strings.TrimSpace(request.Header.Get("TraceId"))
	if traceID != "" {
		return util.NewTraceContextWith(traceID)
	}
	return util.NewTraceContext()
}

func writeSuccess(writer http.ResponseWriter, data any) {
	writeResponse(writer, response{Success: true, Data: data})
}

func writeError(writer http.ResponseWriter, message string) {
	writeResponse(writer, response{Success: false, Message: message, Data: ""})
}

func writeResponse(writer http.ResponseWriter, value response) {
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}
