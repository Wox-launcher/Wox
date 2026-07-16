//go:build wox_automation

package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type fakeController struct {
	actionID    string
	action      woxui.AccessibilityAction
	actionValue string
}

func (f *fakeController) AutomationSnapshot() woxwidget.AutomationSnapshot {
	return woxwidget.AutomationSnapshot{Tree: woxui.AccessibilityTree{
		Generation: 4,
		RootIDs:    []woxui.AccessibilityNodeID{1},
		Nodes:      []woxui.AccessibilityNode{{ID: 1, AutomationID: "launcher.query", Role: woxui.AccessibilityRoleTextField}},
	}}
}

func (f *fakeController) WaitForAutomationChange(context.Context, uint64) (woxwidget.AutomationSnapshot, error) {
	snapshot := f.AutomationSnapshot()
	snapshot.Tree.Generation = 5
	return snapshot, nil
}

func (f *fakeController) PerformAutomationAction(automationID string, action woxui.AccessibilityAction, value string) error {
	f.actionID = automationID
	f.action = action
	f.actionValue = value
	return nil
}

func (*fakeController) PressAutomationKey(woxui.Key, woxui.KeyModifiers) error { return nil }
func (*fakeController) EnterAutomationText(string) error                       { return nil }
func (*fakeController) ShowAutomationWindow() error                            { return nil }
func (*fakeController) HideAutomationWindow() error                            { return nil }
func (*fakeController) AutomationWindowBounds() (woxui.Rect, error) {
	return woxui.Rect{X: 10, Y: 20, Width: 760, Height: 480}, nil
}
func (*fakeController) SetAutomationWindowBounds(woxui.Rect) error { return nil }
func (*fakeController) CaptureAutomationWindow(string) error       { return nil }

func TestHandlerRequiresTokenAndReturnsSemantics(t *testing.T) {
	t.Parallel()

	handler := newHandler(&fakeController{}, "secret-token")
	unauthorized := rpcRequestRecorder(t, handler, "", `{"jsonrpc":"2.0","id":1,"method":"semantics.snapshot"}`)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status, got %d", unauthorized.Code)
	}

	authorized := rpcRequestRecorder(t, handler, "secret-token", `{"jsonrpc":"2.0","id":1,"method":"semantics.snapshot"}`)
	var response struct {
		Result struct {
			Tree woxui.AccessibilityTree `json:"Tree"`
		} `json:"result"`
	}
	if err := json.Unmarshal(authorized.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Result.Tree.Generation != 4 || len(response.Result.Tree.Nodes) != 1 {
		t.Fatalf("unexpected semantics snapshot: %+v", response.Result.Tree)
	}
}

func TestHandlerDispatchesSemanticActionAndRejectsUnknownMethod(t *testing.T) {
	t.Parallel()

	controller := &fakeController{}
	handler := newHandler(controller, "secret-token")
	actionResponse := rpcRequestRecorder(t, handler, "secret-token", `{"jsonrpc":"2.0","id":"action","method":"semantics.perform","params":{"automationId":"launcher.query","action":"set_value","value":"hello"}}`)
	if actionResponse.Code != http.StatusOK {
		t.Fatalf("expected action status 200, got %d", actionResponse.Code)
	}
	if controller.actionID != "launcher.query" || controller.action != woxui.AccessibilityActionSetValue || controller.actionValue != "hello" {
		t.Fatalf("unexpected action call: id=%q action=%q value=%q", controller.actionID, controller.action, controller.actionValue)
	}

	unknownResponse := rpcRequestRecorder(t, handler, "secret-token", `{"jsonrpc":"2.0","id":2,"method":"core.business-route"}`)
	var unknown rpcResponse
	if err := json.Unmarshal(unknownResponse.Body.Bytes(), &unknown); err != nil {
		t.Fatalf("decode unknown-method response: %v", err)
	}
	if unknown.Error == nil || unknown.Error.Code != -32601 {
		t.Fatalf("expected method-not-found response, got %+v", unknown)
	}
}

func rpcRequestRecorder(t *testing.T, handler http.Handler, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
