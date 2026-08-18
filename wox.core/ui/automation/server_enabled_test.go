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
	actionID       string
	action         woxui.AccessibilityAction
	actionValue    string
	pointer        woxui.PointerEvent
	settingsPath   string
	selectionText  string
	reset          bool
	metricsReset   bool
	frameRequested bool
	repaintMode    woxwidget.RepaintDebugMode
	deviceRemoved  bool
	keyHandled     bool
}

func (*fakeController) AutomationFrameMetrics() (woxui.FrameMetricsSnapshot, error) {
	return woxui.FrameMetricsSnapshot{FrameCount: 7, PresentedFrameCount: 6}, nil
}

func (f *fakeController) ResetAutomationFrameMetrics() error {
	f.metricsReset = true
	return nil
}

func (f *fakeController) RequestAutomationFrame() error {
	f.frameRequested = true
	return nil
}

func (f *fakeController) SetAutomationRepaintDebugMode(mode woxwidget.RepaintDebugMode) error {
	f.repaintMode = mode
	return nil
}

func (f *fakeController) SimulateAutomationRendererDeviceRemoved() error {
	f.deviceRemoved = true
	return nil
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

func (f *fakeController) DispatchAutomationPointer(event woxui.PointerEvent) error {
	f.pointer = event
	return nil
}

func (f *fakeController) PressAutomationKey(woxui.Key, woxui.KeyModifiers) (bool, error) {
	return f.keyHandled, nil
}
func (*fakeController) EnterAutomationText(string) error { return nil }
func (f *fakeController) ResetAutomationState() error {
	f.reset = true
	return nil
}
func (*fakeController) ShowAutomationWindow() error { return nil }
func (f *fakeController) OpenAutomationSelectionQuery(text string) error {
	f.selectionText = text
	return nil
}
func (*fakeController) OpenAutomationExplorerQuery(string) error { return nil }
func (*fakeController) SetAutomationFocusInstance(string) error  { return nil }
func (f *fakeController) OpenAutomationSettings(path string) error {
	f.settingsPath = path
	return nil
}
func (*fakeController) HideAutomationWindow() error { return nil }
func (*fakeController) AutomationWindowState(instanceName string) (WindowState, error) {
	return WindowState{Exists: instanceName == "selection", Visible: true, BlurReady: true, Lifecycle: "visible"}, nil
}
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
	metricsResponse := rpcRequestRecorder(t, handler, "secret-token", `{"jsonrpc":"2.0","id":"metrics","method":"render.metrics"}`)
	var metricsResult struct {
		Result woxui.FrameMetricsSnapshot `json:"result"`
	}
	if err := json.Unmarshal(metricsResponse.Body.Bytes(), &metricsResult); err != nil {
		t.Fatalf("decode metrics response: %v", err)
	}
	if metricsResult.Result.FrameCount != 7 || metricsResult.Result.PresentedFrameCount != 6 {
		t.Fatalf("unexpected frame metrics: %+v", metricsResult.Result)
	}
	metricsResetResponse := rpcRequestRecorder(t, handler, "secret-token", `{"jsonrpc":"2.0","id":"metrics-reset","method":"render.metrics.reset"}`)
	if metricsResetResponse.Code != http.StatusOK || !controller.metricsReset {
		t.Fatalf("frame metrics reset was not dispatched: status=%d reset=%v", metricsResetResponse.Code, controller.metricsReset)
	}
	frameResponse := rpcRequestRecorder(t, handler, "secret-token", `{"jsonrpc":"2.0","id":"frame","method":"render.invalidate"}`)
	if frameResponse.Code != http.StatusOK || !controller.frameRequested {
		t.Fatalf("frame invalidation was not dispatched: status=%d requested=%v", frameResponse.Code, controller.frameRequested)
	}
	repaintResponse := rpcRequestRecorder(t, handler, "secret-token", `{"jsonrpc":"2.0","id":"repaint","method":"render.repaint_debug","params":{"mode":"verify"}}`)
	if repaintResponse.Code != http.StatusOK || controller.repaintMode != woxwidget.RepaintDebugVerify {
		t.Fatalf("repaint mode was not dispatched: status=%d mode=%q", repaintResponse.Code, controller.repaintMode)
	}
	deviceRemovedResponse := rpcRequestRecorder(t, handler, "secret-token", `{"jsonrpc":"2.0","id":"device-removed","method":"render.simulate_device_removed"}`)
	if deviceRemovedResponse.Code != http.StatusOK || !controller.deviceRemoved {
		t.Fatalf("renderer device removal was not dispatched: status=%d simulated=%v", deviceRemovedResponse.Code, controller.deviceRemoved)
	}
	actionResponse := rpcRequestRecorder(t, handler, "secret-token", `{"jsonrpc":"2.0","id":"action","method":"semantics.perform","params":{"automationId":"launcher.query","action":"set_value","value":"hello"}}`)
	if actionResponse.Code != http.StatusOK {
		t.Fatalf("expected action status 200, got %d", actionResponse.Code)
	}
	if controller.actionID != "launcher.query" || controller.action != woxui.AccessibilityActionSetValue || controller.actionValue != "hello" {
		t.Fatalf("unexpected action call: id=%q action=%q value=%q", controller.actionID, controller.action, controller.actionValue)
	}

	pointerResponse := rpcRequestRecorder(t, handler, "secret-token", `{"jsonrpc":"2.0","id":"pointer","method":"input.pointer","params":{"Kind":0,"Position":{"X":120.5,"Y":48.25}}}`)
	if pointerResponse.Code != http.StatusOK {
		t.Fatalf("expected pointer status 200, got %d", pointerResponse.Code)
	}
	if controller.pointer.Kind != woxui.PointerMove || controller.pointer.Position != (woxui.Point{X: 120.5, Y: 48.25}) {
		t.Fatalf("unexpected pointer call: %+v", controller.pointer)
	}
	controller.keyHandled = true
	keyResponse := rpcRequestRecorder(t, handler, "secret-token", `{"jsonrpc":"2.0","id":"key","method":"input.key","params":{"key":"v","modifiers":8}}`)
	var keyResult struct {
		Result bool `json:"result"`
	}
	if err := json.Unmarshal(keyResponse.Body.Bytes(), &keyResult); err != nil {
		t.Fatalf("decode key response: %v", err)
	}
	if keyResponse.Code != http.StatusOK || !keyResult.Result {
		t.Fatalf("key handled result was not returned: status=%d handled=%v", keyResponse.Code, keyResult.Result)
	}

	settingsResponse := rpcRequestRecorder(t, handler, "secret-token", `{"jsonrpc":"2.0","id":"settings","method":"window.open_settings","params":{"path":"/appearance"}}`)
	if settingsResponse.Code != http.StatusOK {
		t.Fatalf("expected settings status 200, got %d", settingsResponse.Code)
	}
	if controller.settingsPath != "/appearance" {
		t.Fatalf("unexpected settings path %q", controller.settingsPath)
	}
	selectionResponse := rpcRequestRecorder(t, handler, "secret-token", `{"jsonrpc":"2.0","id":"selection","method":"window.open_selection_query","params":{"text":"selected text"}}`)
	if selectionResponse.Code != http.StatusOK || controller.selectionText != "selected text" {
		t.Fatalf("selection query was not dispatched: status=%d text=%q", selectionResponse.Code, controller.selectionText)
	}
	stateResponse := rpcRequestRecorder(t, handler, "secret-token", `{"jsonrpc":"2.0","id":"state","method":"window.state","params":{"instanceName":"selection"}}`)
	var stateResult struct {
		Result WindowState `json:"result"`
	}
	if err := json.Unmarshal(stateResponse.Body.Bytes(), &stateResult); err != nil {
		t.Fatalf("decode window state response: %v", err)
	}
	if stateResponse.Code != http.StatusOK || !stateResult.Result.Exists || !stateResult.Result.BlurReady || stateResult.Result.Lifecycle != "visible" {
		t.Fatalf("unexpected selection window state: status=%d state=%+v", stateResponse.Code, stateResult.Result)
	}

	resetResponse := rpcRequestRecorder(t, handler, "secret-token", `{"jsonrpc":"2.0","id":"reset","method":"suite.reset"}`)
	if resetResponse.Code != http.StatusOK || !controller.reset {
		t.Fatalf("suite reset was not dispatched: status=%d reset=%v", resetResponse.Code, controller.reset)
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
