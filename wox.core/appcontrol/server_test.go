package appcontrol

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"wox/util"
)

func TestHandlerExposesOnlyProcessControlRoutes(t *testing.T) {
	t.Parallel()

	handler := NewHandler(Handlers{
		Show: func(context.Context) error { return nil },
		DeepLink: func(context.Context, string) error {
			return nil
		},
		EnableDiagnosticsAndRestart: func(context.Context) (any, error) {
			return map[string]bool{"Enabled": true}, nil
		},
	})

	for _, testCase := range []struct {
		name string
		path string
		body string
	}{
		{name: "ping", path: "/ping"},
		{name: "show", path: "/show"},
		{name: "deeplink", path: "/deeplink", body: `{"deeplink":"wox://setting"}`},
		{name: "diagnostics", path: "/diagnostics/monitor/enable-restart"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, testCase.path, strings.NewReader(testCase.body))
			responseRecorder := httptest.NewRecorder()
			handler.ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != http.StatusOK {
				t.Fatalf("expected status 200, got %d", responseRecorder.Code)
			}
			var payload response
			if err := json.Unmarshal(responseRecorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if !payload.Success {
				t.Fatalf("expected success response, got %q", payload.Message)
			}
		})
	}

	request := httptest.NewRequest(http.MethodPost, "/setting/wox", nil)
	responseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusNotFound {
		t.Fatalf("business route must not be externally reachable, got status %d", responseRecorder.Code)
	}
}

func TestHandlerValidatesDeepLinkAndPreservesTraceID(t *testing.T) {
	t.Parallel()

	var receivedDeepLink string
	var receivedTraceID string
	handler := NewHandler(Handlers{
		DeepLink: func(ctx context.Context, deepLink string) error {
			receivedDeepLink = deepLink
			receivedTraceID = util.GetContextTraceId(ctx)
			return nil
		},
	})

	invalidRequest := httptest.NewRequest(http.MethodPost, "/deeplink", strings.NewReader(`{"deeplink":"  "}`))
	invalidResponse := httptest.NewRecorder()
	handler.ServeHTTP(invalidResponse, invalidRequest)
	var invalidPayload response
	if err := json.Unmarshal(invalidResponse.Body.Bytes(), &invalidPayload); err != nil {
		t.Fatalf("decode invalid response: %v", err)
	}
	if invalidPayload.Success || invalidPayload.Message != "deeplink is empty" {
		t.Fatalf("expected empty deeplink error, got %+v", invalidPayload)
	}

	request := httptest.NewRequest(http.MethodPost, "/deeplink", strings.NewReader(`{"deeplink":"  wox://setting  "}`))
	request.Header.Set("TraceId", "trace-from-secondary-instance")
	responseRecorder := httptest.NewRecorder()
	handler.ServeHTTP(responseRecorder, request)

	if receivedDeepLink != "wox://setting" {
		t.Fatalf("expected trimmed deeplink, got %q", receivedDeepLink)
	}
	if receivedTraceID != "trace-from-secondary-instance" {
		t.Fatalf("expected trace id to be preserved, got %q", receivedTraceID)
	}
}
