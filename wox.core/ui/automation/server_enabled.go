//go:build wox_automation

package automation

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	woxui "wox/ui/runtime"
)

const (
	automationTokenEnvironment    = "WOX_AUTOMATION_TOKEN"
	automationInfoFileEnvironment = "WOX_AUTOMATION_INFO_FILE"
	maxAutomationRequestBytes     = 1024 * 1024
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Start runs the authenticated automation server for wox_automation builds.
func Start(ctx context.Context, controller Controller) (Info, error) {
	if controller == nil {
		return Info{}, errors.New("automation controller is required")
	}
	token := strings.TrimSpace(os.Getenv(automationTokenEnvironment))
	if token == "" {
		var err error
		token, err = newToken()
		if err != nil {
			return Info{}, err
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Info{}, fmt.Errorf("listen for automation: %w", err)
	}
	info := Info{Address: "http://" + listener.Addr().String(), Token: token}
	if err := writeInfoFile(info); err != nil {
		_ = listener.Close()
		return Info{}, err
	}

	server := &http.Server{
		Handler:           newHandler(controller, token),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "Wox automation server stopped: %v\n", err)
		}
	}()
	return info, nil
}

func newHandler(controller Controller, token string) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		provided := strings.TrimSpace(strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer "))
		if len(provided) != len(token) || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		request.Body = http.MaxBytesReader(writer, request.Body, maxAutomationRequestBytes)
		var call rpcRequest
		if err := json.NewDecoder(request.Body).Decode(&call); err != nil {
			writeRPCError(writer, nil, -32700, "invalid JSON-RPC request")
			return
		}
		if call.JSONRPC != "2.0" || call.Method == "" {
			writeRPCError(writer, call.ID, -32600, "invalid JSON-RPC request")
			return
		}
		result, rpcErr := dispatch(request.Context(), controller, call.Method, call.Params)
		if rpcErr != nil {
			writeRPCError(writer, call.ID, rpcErr.Code, rpcErr.Message)
			return
		}
		writeRPCResponse(writer, rpcResponse{JSONRPC: "2.0", ID: call.ID, Result: result})
	})
}

func dispatch(ctx context.Context, controller Controller, method string, rawParams json.RawMessage) (any, *rpcError) {
	switch method {
	case "semantics.snapshot":
		return controller.AutomationSnapshot(), nil
	case "semantics.wait":
		var params struct {
			AfterGeneration uint64 `json:"afterGeneration"`
			TimeoutMS       int    `json:"timeoutMs"`
		}
		if err := decodeParams(rawParams, &params); err != nil {
			return nil, invalidParams(err)
		}
		if params.TimeoutMS <= 0 || params.TimeoutMS > 30000 {
			params.TimeoutMS = 5000
		}
		waitCtx, cancel := context.WithTimeout(ctx, time.Duration(params.TimeoutMS)*time.Millisecond)
		defer cancel()
		snapshot, err := controller.WaitForAutomationChange(waitCtx, params.AfterGeneration)
		return resultOrError(snapshot, err)
	case "semantics.perform":
		var params struct {
			AutomationID string                    `json:"automationId"`
			Action       woxui.AccessibilityAction `json:"action"`
			Value        string                    `json:"value"`
		}
		if err := decodeParams(rawParams, &params); err != nil {
			return nil, invalidParams(err)
		}
		if params.AutomationID == "" || params.Action == "" {
			return nil, invalidParams(errors.New("automationId and action are required"))
		}
		return resultOrError(true, controller.PerformAutomationAction(params.AutomationID, params.Action, params.Value))
	case "input.key":
		var params struct {
			Key       woxui.Key          `json:"key"`
			Modifiers woxui.KeyModifiers `json:"modifiers"`
		}
		if err := decodeParams(rawParams, &params); err != nil {
			return nil, invalidParams(err)
		}
		if params.Key == "" {
			return nil, invalidParams(errors.New("key is required"))
		}
		return resultOrError(true, controller.PressAutomationKey(params.Key, params.Modifiers))
	case "input.text":
		var params struct {
			Text string `json:"text"`
		}
		if err := decodeParams(rawParams, &params); err != nil {
			return nil, invalidParams(err)
		}
		return resultOrError(true, controller.EnterAutomationText(params.Text))
	case "window.show":
		return resultOrError(true, controller.ShowAutomationWindow())
	case "window.hide":
		return resultOrError(true, controller.HideAutomationWindow())
	case "window.bounds":
		bounds, err := controller.AutomationWindowBounds()
		return resultOrError(bounds, err)
	case "window.set_bounds":
		var bounds woxui.Rect
		if err := decodeParams(rawParams, &bounds); err != nil {
			return nil, invalidParams(err)
		}
		if bounds.Width <= 0 || bounds.Height <= 0 {
			return nil, invalidParams(errors.New("window bounds must have a positive size"))
		}
		return resultOrError(true, controller.SetAutomationWindowBounds(bounds))
	case "window.capture":
		var params struct {
			Path string `json:"path"`
		}
		if err := decodeParams(rawParams, &params); err != nil {
			return nil, invalidParams(err)
		}
		if strings.TrimSpace(params.Path) == "" {
			return nil, invalidParams(errors.New("capture path is required"))
		}
		return resultOrError(true, controller.CaptureAutomationWindow(params.Path))
	default:
		return nil, &rpcError{Code: -32601, Message: "method not found"}
	}
}

func decodeParams(raw json.RawMessage, target any) error {
	if len(raw) == 0 || string(raw) == "null" {
		raw = []byte("{}")
	}
	return json.Unmarshal(raw, target)
}

func invalidParams(err error) *rpcError {
	return &rpcError{Code: -32602, Message: err.Error()}
}

func resultOrError(result any, err error) (any, *rpcError) {
	if err != nil {
		return nil, &rpcError{Code: -32000, Message: err.Error()}
	}
	return result, nil
}

func writeRPCError(writer http.ResponseWriter, id json.RawMessage, code int, message string) {
	writeRPCResponse(writer, rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: message}})
}

func writeRPCResponse(writer http.ResponseWriter, response rpcResponse) {
	writer.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(writer).Encode(response); err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
	}
}

func newToken() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate automation token: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func writeInfoFile(info Info) error {
	path := strings.TrimSpace(os.Getenv(automationInfoFileEnvironment))
	if path == "" {
		return nil
	}
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary automation info file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary automation info file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary automation info file: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish automation info file: %w", err)
	}
	return nil
}
