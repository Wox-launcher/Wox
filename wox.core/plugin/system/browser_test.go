package system

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"wox/plugin"
	"wox/util"

	"github.com/gorilla/websocket"
)

type browserTestAPI struct {
	plugin.API
}

func (browserTestAPI) Log(context.Context, plugin.LogLevel, string) {}

func TestBrowserPluginWebsocketLifecycle(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, dataDir)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(dataDir, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatalf("init test location: %v", err)
	}

	bp := &BrowserPlugin{api: browserTestAPI{}}
	server := httptest.NewServer(bp.newWebsocketMux())
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial ws failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for !bp.IsExtensionConnected() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !bp.IsExtensionConnected() {
		t.Fatal("expected IsExtensionConnected to be true after dial")
	}

	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"method":"ping","data":""}`)); err != nil {
		t.Fatalf("write ping failed: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, pong, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read pong failed: %v", err)
	}
	if string(pong) != `{"method":"pong"}` {
		t.Fatalf("pong = %q, want %q", string(pong), `{"method":"pong"}`)
	}

	tabsPayload, _ := json.Marshal([]browserTab{{
		TabId:    42,
		WindowId: 7,
		TabIndex: 1,
		Title:    "Wox",
		Url:      "https://example.com",
	}})
	tabsMsg, _ := json.Marshal(websocketMsg{Method: "tabs", Data: string(tabsPayload)})
	if err := conn.WriteMessage(websocket.TextMessage, tabsMsg); err != nil {
		t.Fatalf("write tabs failed: %v", err)
	}

	deadline = time.Now().Add(2 * time.Second)
	for len(bp.GetOpenedTabs()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	tabs := bp.GetOpenedTabs()
	if len(tabs) != 1 || tabs[0].TabId != 42 || tabs[0].Url != "https://example.com" {
		t.Fatalf("GetOpenedTabs = %+v", tabs)
	}

	if err := bp.HighlightTab(42, 7, 1); err != nil {
		t.Fatalf("HighlightTab failed: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, highlightMsg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read highlightTab failed: %v", err)
	}
	if !strings.Contains(string(highlightMsg), `"method":"highlightTab"`) {
		t.Fatalf("highlightMsg = %q", string(highlightMsg))
	}

	if err := bp.OpenUrlViaExtension("https://wox-launcher.github.io"); err != nil {
		t.Fatalf("OpenUrlViaExtension failed: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, openMsg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read openUrl failed: %v", err)
	}
	if !strings.Contains(string(openMsg), `"method":"openUrl"`) {
		t.Fatalf("openMsg = %q", string(openMsg))
	}

	_ = conn.Close()
	deadline = time.Now().Add(2 * time.Second)
	for (bp.IsExtensionConnected() || len(bp.GetOpenedTabs()) > 0) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if bp.IsExtensionConnected() || len(bp.GetOpenedTabs()) != 0 {
		t.Fatalf("expected disconnect cleanup, connected=%v tabs=%v", bp.IsExtensionConnected(), bp.GetOpenedTabs())
	}
}
