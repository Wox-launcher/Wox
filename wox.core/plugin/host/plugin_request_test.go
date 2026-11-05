package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"wox/common"
	"wox/network"
	"wox/plugin"
	"wox/util"

	"github.com/gorilla/websocket"
)

type requestTestRuntime struct{ plugin.Host }

func (requestTestRuntime) GetRuntime(context.Context) plugin.Runtime {
	return plugin.PLUGIN_RUNTIME_NODEJS
}

// newPluginRequestPeer captures real wire replies and acknowledges reverse callback RPCs.
func newPluginRequestPeer(t *testing.T) (*WebsocketHost, <-chan JsonRpcResponse, <-chan JsonRpcRequest) {
	t.Helper()
	previous := network.IsOffline()
	network.Default.SetOffline(false)
	t.Cleanup(func() { network.Default.SetOffline(previous) })
	responses := make(chan JsonRpcResponse, 64)
	callbacks := make(chan JsonRpcRequest, 16)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var envelope struct{ Type JsonRpcType }
			if json.Unmarshal(data, &envelope) != nil {
				return
			}
			if envelope.Type == JsonRpcTypeRequest {
				var callback JsonRpcRequest
				if json.Unmarshal(data, &callback) != nil {
					return
				}
				callbacks <- callback
				if conn.WriteJSON(JsonRpcResponse{Id: callback.Id, Method: callback.Method, Type: JsonRpcTypeResponse}) != nil {
					return
				}
			} else {
				var response JsonRpcResponse
				if json.Unmarshal(data, &response) != nil {
					return
				}
				responses <- response
			}
		}
	}))
	t.Cleanup(server.Close)
	client := util.NewWebsocketClient("ws" + strings.TrimPrefix(server.URL, "http"))
	host := &WebsocketHost{ws: client, host: requestTestRuntime{}, requestMap: util.NewHashMap[string, chan JsonRpcResponse]()}
	client.OnMessage(t.Context(), func(data []byte) { host.onMessage(string(data)) })
	if err := client.Connect(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close(context.Background()) })
	return host, responses, callbacks
}

// receivePluginResponse checks correlation as well as delivery so duplicate replies fail the next call.
func receivePluginResponse(t *testing.T, responses <-chan JsonRpcResponse, request JsonRpcRequest) JsonRpcResponse {
	t.Helper()
	select {
	case response := <-responses:
		if response.Id != request.Id || response.Method != request.Method || response.Type != JsonRpcTypeResponse {
			t.Fatalf("response envelope = %+v, request = %+v", response, request)
		}
		return response
	case <-time.After(3 * time.Second):
		t.Fatalf("request %s/%s did not receive a response", request.Method, request.Id)
		return JsonRpcResponse{}
	}
}

// TestPluginRequestInvalidArgumentsReply covers every previously silent required-parameter branch.
func TestPluginRequestInvalidArgumentsReply(t *testing.T) {
	host, responses, _ := newPluginRequestPeer(t)
	caller := &plugin.Instance{API: &requestTestAPI{}}
	cases := []struct {
		method string
		params map[string]string
		want   string
	}{
		{"UnknownMethod", nil, "unknown plugin method"},
		{"GetSetting", nil, `"key"`},
		{"SaveSetting", nil, `"key"`},
		{"SaveSetting", map[string]string{"key": ""}, `"value"`},
		{"SaveSetting", map[string]string{"key": "", "value": ""}, `"isPlatformSpecific"`},
		{"Notify", nil, `"message"`},
		{"PushAttention", nil, `"request"`},
		{"ShowToolbarMsg", nil, `"msg"`},
		{"ClearToolbarMsg", nil, `"toolbarMsgId"`},
		{"Log", nil, `"msg"`},
		{"Log", map[string]string{"msg": ""}, `"level"`},
		{"GetTranslation", nil, `"key"`},
		{"GetUpdatableResult", nil, `"resultId"`},
		{"UpdateResult", nil, `"result"`},
		{"RemoveResult", nil, `"option"`},
		{"RemoveResult", map[string]string{"option": "{"}, "unmarshal remove result option"},
		{"PushResults", nil, `"query"`},
		{"PushResults", map[string]string{"query": "{}"}, `"results"`},
		{"ExecuteToolbarMsgAction", nil, `"toolbarMsgId"`},
		{"ExecuteToolbarMsgAction", map[string]string{"toolbarMsgId": ""}, `"actionId"`},
		{"ChangeQuery", map[string]string{"queryType": "selection", "querySelection": "{"}, "unmarshal selection"},
		{"ShowToolbarMsg", map[string]string{"msg": "{"}, "unmarshal toolbar msg"},
		{"UpdateResult", map[string]string{"result": "{"}, "unmarshal updatable result"},
		{"PushResults", map[string]string{"query": "{", "results": "[]"}, "unmarshal query"},
		{"PushResults", map[string]string{"query": "{}", "results": "{"}, "unmarshal results"},
		{"RegisterQueryCommands", map[string]string{"commands": "{"}, "unmarshal commands"},
		{"Screenshot", map[string]string{"option": "{"}, "unmarshal screenshot option"},
		{"GetThemeColors", map[string]string{"option": "{"}, "unmarshal theme colors option"},
		{"SetSetting", map[string]string{"option": "{"}, "unmarshal SetSetting option"},
		{"PushAttention", map[string]string{"request": "{"}, "unmarshal push attention request"},
	}
	for _, method := range []string{"OnPluginSettingChanged", "OnGetDynamicSetting", "OnDeepLink", "OnUnload", "OnEnterPluginQuery", "OnLeavePluginQuery", "OnDragOut", "OnMRURestore"} {
		cases = append(cases, struct {
			method string
			params map[string]string
			want   string
		}{method, nil, `"callbackId"`})
	}
	for index, test := range cases {
		t.Run(fmt.Sprintf("%s/%d", test.method, index), func(t *testing.T) {
			request := JsonRpcRequest{Id: fmt.Sprint(index), Method: test.method, Params: test.params}
			host.respondToPluginRequest(t.Context(), request, caller)
			response := receivePluginResponse(t, responses, request)
			if !strings.Contains(response.Error, test.want) || response.Result != nil {
				t.Fatalf("response = %+v, want error containing %q and null result", response, test.want)
			}
		})
	}
	select {
	case extra := <-responses:
		t.Fatalf("duplicate response: %+v", extra)
	case <-time.After(50 * time.Millisecond):
	}
}

type requestTestAPI struct {
	plugin.API
	savedKey, savedValue, session, queryID string
	savedPlatform                          bool
	query                                  common.PlainQuery
	refresh                                plugin.RefreshQueryParam
	copy                                   plugin.CopyParams
	screenshot                             plugin.ScreenshotOption
	result                                 *plugin.UpdatableResult
	remove                                 plugin.RemoveResultOption
	settingChanged                         func(context.Context, string, string)
	toast                                  plugin.ShowToastMsgOption
	toolbar                                plugin.ToolbarMsg
}

func (a *requestTestAPI) ShowToolbarMsg(_ context.Context, msg plugin.ToolbarMsg) {
	a.toolbar = msg
}

// TestToolbarWireKeepsPublicActionIDsSeparateFromManagedHostSnapshots preserves legacy payloads.
func TestToolbarWireKeepsPublicActionIDsSeparateFromManagedHostSnapshots(t *testing.T) {
	api := &requestTestAPI{}
	instance := &plugin.Instance{API: api}
	host := &WebsocketHost{}
	msg := `{"Id":"status","Title":"Ready","HostSnapshotId":"snapshot","Actions":[{"Id":"run","Name":"Run"}]}`
	for _, managed := range []bool{false, true} {
		request := JsonRpcRequest{Params: common.ContextData{"msg": msg}}
		if managed {
			request.Params["managedToolbar"] = "true"
		}
		if _, err := host.handleShowToolbarMsg(context.Background(), instance, request); err != nil {
			t.Fatal(err)
		}
		if api.toolbar.Id != "status" || len(api.toolbar.Actions) != 1 || api.toolbar.Actions[0].Id != "run" {
			t.Fatalf("toolbar payload = %+v", api.toolbar)
		}
		want := ""
		if managed {
			want = "snapshot"
		}
		if api.toolbar.HostSnapshotId != want {
			t.Fatalf("callback token = %q, want %q", api.toolbar.HostSnapshotId, want)
		}
	}
}

// TestToolbarCallbackReleaseUsesHostRPCWhileOffline keeps local cleanup independent of network policy.
func TestToolbarCallbackReleaseUsesHostRPCWhileOffline(t *testing.T) {
	host, _, callbacks := newPluginRequestPeer(t)
	network.Default.SetOffline(true)
	proxy := NewWebsocketPlugin(plugin.Metadata{Id: "owner", Name: "Owner"}, host)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		proxy.RetireToolbarSnapshot(ctx, "snapshot")
		close(done)
	}()
	select {
	case request := <-callbacks:
		if request.Method != "retireToolbarSnapshot" || request.PluginId != "owner" || request.Params["HostSnapshotId"] != "snapshot" {
			t.Fatalf("unexpected cleanup request: %+v", request)
		}
	case <-ctx.Done():
		t.Fatal("toolbar cleanup was not sent")
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("toolbar cleanup did not complete")
	}
}

func (a *requestTestAPI) ShowToastMsg(_ context.Context, option plugin.ShowToastMsgOption) plugin.ShowToastMsgResult {
	a.toast = option
	return plugin.ShowToastMsgResult{Success: true}
}

func (a *requestTestAPI) GetSetting(_ context.Context, key string) string {
	if key == "panic" {
		panic("test API failure")
	}
	return "value:" + key
}

func (a *requestTestAPI) GetTranslation(_ context.Context, key string) string {
	return "translated:" + key
}

// SaveSetting records empty values and originating query identity without touching persisted settings.
func (a *requestTestAPI) SaveSetting(ctx context.Context, key, value string, platform bool) {
	a.savedKey, a.savedValue, a.savedPlatform = key, value, platform
	a.session, a.queryID = util.GetContextSessionId(ctx), util.GetContextQueryId(ctx)
}

func (a *requestTestAPI) ChangeQuery(_ context.Context, query common.PlainQuery) { a.query = query }
func (a *requestTestAPI) RefreshQuery(_ context.Context, option plugin.RefreshQueryParam) {
	a.refresh = option
}
func (a *requestTestAPI) Copy(_ context.Context, option plugin.CopyParams) { a.copy = option }
func (a *requestTestAPI) Screenshot(_ context.Context, option plugin.ScreenshotOption) plugin.ScreenshotResult {
	a.screenshot = option
	return plugin.ScreenshotResult{Success: true, ScreenshotPath: "capture.png"}
}
func (a *requestTestAPI) RemoveResult(_ context.Context, option plugin.RemoveResultOption) plugin.RemoveResultResult {
	a.remove = option
	return plugin.RemoveResultResult{Success: true}
}
func (a *requestTestAPI) GetUpdatableResult(context.Context, string) *plugin.UpdatableResult {
	return a.result
}
func (a *requestTestAPI) OnSettingChanged(_ context.Context, callback func(context.Context, string, string)) {
	a.settingChanged = callback
}
func (a *requestTestAPI) AIChatStream(context.Context, common.Model, []common.Conversation, common.ChatOptions, common.ChatStreamFunc) error {
	return errors.New("stream could not start")
}

// TestPluginRequestPreservesLegacyWireValues covers defaults, aliases, optional decoration and nested JSON strings.
func TestPluginRequestPreservesLegacyWireValues(t *testing.T) {
	host, responses, _ := newPluginRequestPeer(t)
	api := &requestTestAPI{}
	caller := &plugin.Instance{API: api}
	call := func(method string, params map[string]string) any {
		t.Helper()
		request := JsonRpcRequest{Id: method, Method: method, SessionId: "launcher-2", QueryId: "query-7", Params: params}
		host.respondToPluginRequest(t.Context(), request, caller)
		response := receivePluginResponse(t, responses, request)
		if response.Error != "" {
			t.Fatalf("%s: %s", method, response.Error)
		}
		return response.Result
	}
	if got := call("SaveSetting", map[string]string{"key": "", "value": "", "isPlatformSpecific": "TRUE"}); got != "" || api.savedKey != "" || api.savedValue != "" || !api.savedPlatform || api.session != "launcher-2" || api.queryID != "query-7" {
		t.Fatalf("legacy save acknowledgement or context changed: result=%#v api=%+v", got, api)
	}
	if got := call("Notify", map[string]string{"message": "legacy message"}); got != "" || api.toast.Message != "legacy message" {
		t.Fatalf("legacy Notify did not choose toast: result=%#v option=%+v", got, api.toast)
	}
	if got := call("GetSetting", map[string]string{"key": ""}); got != "value:" {
		t.Fatalf("empty key response = %#v", got)
	}
	call("ChangeQuery", map[string]string{"queryType": "input", "queryText": "", "queryHint": "{"})
	if api.query.QueryType != plugin.QueryTypeInput || api.query.QueryText != "" || api.query.QueryHint != nil {
		t.Fatalf("optional malformed hint changed query: %+v", api.query)
	}
	toast := call("ShowToastMsg", map[string]string{"option": `{"Message":"hello","Title":"title","Action":{"Name":"Open","CallbackId":"action-id","ContextData":{"path":"file"}}}`}).(map[string]any)
	if toast["Success"] != true || api.toast.Message != "hello" || api.toast.Title != "title" || api.toast.Action == nil || api.toast.Action.Name != "Open" || api.toast.Action.ContextData["path"] != "file" || api.toast.Action.Action == nil || api.toast.Action.Release == nil {
		t.Fatalf("toast option or callback proxy lost: result=%#v option=%+v", toast, api.toast)
	}
	call("RefreshQuery", map[string]string{"preserveSelectedIndex": "true", "SelectedResultId": " row "})
	if !api.refresh.PreserveSelectedIndex || api.refresh.SelectedResultId != "row" {
		t.Fatalf("legacy refresh aliases lost: %+v", api.refresh)
	}
	resultRemove := call("RemoveResult", map[string]string{"option": `{"ResultId":"row","SelectedResultId":"next"}`}).(map[string]any)
	if resultRemove["Success"] != true || api.remove.ResultId != "row" || api.remove.SelectedResultId != "next" {
		t.Fatalf("remove option or result lost: %+v %+v", api.remove, resultRemove)
	}
	call("Copy", map[string]string{"type": "text", "text": "keep text", "woxImage": "{"})
	if api.copy.Text != "keep text" || api.copy.WoxImage != nil {
		t.Fatalf("optional image fallback changed: %+v", api.copy)
	}
	for _, raw := range []string{"", " ", "null", "{}"} {
		result := call("Screenshot", map[string]string{"option": raw}).(map[string]any)
		if api.screenshot != (plugin.ScreenshotOption{}) || result["Success"] != true || result["ScreenshotPath"] != "capture.png" {
			t.Fatalf("screenshot defaults or response changed: %+v %+v", api.screenshot, result)
		}
	}
	if got := call("GetUpdatableResult", map[string]string{"resultId": "missing"}); got != nil {
		t.Fatalf("missing result = %#v, want null", got)
	}
	api.result = &plugin.UpdatableResult{Id: "row"}
	got := call("GetUpdatableResult", map[string]string{"resultId": "row"})
	encoded, ok := got.(string)
	var result plugin.UpdatableResult
	if !ok || json.Unmarshal([]byte(encoded), &result) != nil || result.Id != "row" {
		t.Fatalf("updatable result must remain JSON encoded inside a string: %#v", got)
	}
	for _, method := range []string{"RegisterTriggerKeyword", "UnregisterTriggerKeyword", "RegisterPluginTool", "UnregisterPluginTool", "ListPluginTools", "InvokePluginTool"} {
		result := call(method, map[string]string{"option": "{"}).(map[string]any)
		if method == "RegisterTriggerKeyword" || method == "UnregisterTriggerKeyword" {
			if result["Success"] != false {
				t.Fatalf("%s result = %+v", method, result)
			}
		} else if result["Error"] == nil {
			t.Fatalf("%s lost structured error: %+v", method, result)
		}
	}
}

// TestPluginRequestFailureBoundary exercises delivery when routing, execution or serialization fails.
func TestPluginRequestFailureBoundary(t *testing.T) {
	host, responses, _ := newPluginRequestPeer(t)
	request := JsonRpcRequest{Id: "missing", Method: "GetSetting", PluginId: "not-loaded"}
	host.handleRequestFromPlugin(t.Context(), request)
	if response := receivePluginResponse(t, responses, request); !strings.Contains(response.Error, "not found") {
		t.Fatalf("missing caller: %+v", response)
	}
	request.Id, request.Params = "panic", map[string]string{"key": "panic"}
	host.respondToPluginRequest(t.Context(), request, &plugin.Instance{API: &requestTestAPI{}})
	if response := receivePluginResponse(t, responses, request); !strings.Contains(response.Error, "test API failure") {
		t.Fatalf("panic response: %+v", response)
	}
	request.Id = "marshal"
	host.sendPluginResponse(t.Context(), request, make(chan int), nil)
	if response := receivePluginResponse(t, responses, request); !strings.Contains(response.Error, "marshal response") || response.Result != nil {
		t.Fatalf("marshal response: %+v", response)
	}
	request.Id = "offline-after-call"
	network.Default.SetOffline(true)
	host.sendPluginResponse(t.Context(), request, "", nil)
	if response := receivePluginResponse(t, responses, request); response.Error != network.ErrOffline.Error() {
		t.Fatalf("offline response: %+v", response)
	}
	network.Default.SetOffline(false)
	request.Id = "invalid-params"
	host.onMessage(`{"Type":"WOX_JSONRPC_REQUEST","Id":"invalid-params","Method":"GetSetting","Params":{"key":42}}`)
	if response := receivePluginResponse(t, responses, request); !strings.Contains(response.Error, "unmarshal request") {
		t.Fatalf("invalid params: %+v", response)
	}
	// A protocol marker inside an argument must not override the actual envelope type.
	request.Id = "marker-in-value"
	host.onMessage(`{"Type":"WOX_JSONRPC_REQUEST","Id":"marker-in-value","Method":"GetSetting","PluginId":"not-loaded","Params":{"key":"WOX_JSONRPC_SYSTEM_LOG"}}`)
	if response := receivePluginResponse(t, responses, request); !strings.Contains(response.Error, "not found") {
		t.Fatalf("marker routing: %+v", response)
	}
}

// TestPluginRequestCallbacksKeepTheirProtocol verifies callback IDs, context and AI stream failure acknowledgement.
func TestPluginRequestCallbacksKeepTheirProtocol(t *testing.T) {
	host, responses, callbacks := newPluginRequestPeer(t)
	api := &requestTestAPI{}
	caller := &plugin.Instance{Metadata: plugin.Metadata{Id: "owner", Name: "Test"}, API: api}
	request := JsonRpcRequest{Id: "register", Method: "OnPluginSettingChanged", Params: map[string]string{"callbackId": "callback-1"}}
	host.respondToPluginRequest(t.Context(), request, caller)
	if response := receivePluginResponse(t, responses, request); response.Error != "" || response.Result != "" {
		t.Fatalf("registration: %+v", response)
	}
	callbackCtx := util.WithQueryIdContext(util.WithSessionContext(t.Context(), "callback-session"), "callback-query")
	api.settingChanged(callbackCtx, "key", "value")
	select {
	case callback := <-callbacks:
		if callback.Method != "onPluginSettingChange" || callback.Params["CallbackId"] != "callback-1" || callback.Params["Key"] != "key" || callback.Params["Value"] != "value" || callback.SessionId != "callback-session" || callback.QueryId != "callback-query" {
			t.Fatalf("setting callback: %+v", callback)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("missing setting callback")
	}
	request = JsonRpcRequest{Id: "ai", Method: "AIChatStream", Params: map[string]string{"callbackId": "stream-1", "model": "{}", "conversations": "[]", "options": "{}"}}
	host.respondToPluginRequest(t.Context(), request, caller)
	if response := receivePluginResponse(t, responses, request); response.Error != "" || response.Result != "" {
		t.Fatalf("AI acknowledgement: %+v", response)
	}
	select {
	case callback := <-callbacks:
		if callback.Method != "onLLMStream" || callback.Params["CallbackId"] != "stream-1" || callback.Params["StreamType"] != string(common.ChatStreamStatusError) || !strings.Contains(callback.Params["Data"], "stream could not start") {
			t.Fatalf("stream error callback: %+v", callback)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("missing stream error callback")
	}
}

// TestScriptNotifyUsesToast covers the legacy script action without changing its wire format.
func TestScriptNotifyUsesToast(t *testing.T) {
	api := &requestTestAPI{}
	script := &ScriptPlugin{api: api}
	if !script.handleBuiltInAction(t.Context(), "notify", map[string]interface{}{"message": "script message"}) {
		t.Fatal("legacy notify action was not handled")
	}
	if api.toast.Message != "script message" {
		t.Fatalf("toast message = %q", api.toast.Message)
	}
}
