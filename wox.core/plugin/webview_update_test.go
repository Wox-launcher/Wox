package plugin

import (
	"context"
	"testing"
	"wox/util"
)

func TestWebViewScriptContract(t *testing.T) {
	for _, data := range []string{`{"updateMode":"script","script":"tick()","html":"x"}`, `{"updateMode":"script","script":"tick()","url":""}`, `{"updateMode":"bad","script":"tick()"}`, `{"updateMode":"script","script":5}`, `{"script":"tick()"}`} {
		_, command, err := WebViewScriptUpdate(&WoxPreview{PreviewType: WoxPreviewTypeWebView, PreviewData: data})
		if !command || err == nil {
			t.Fatalf("accepted invalid update: %s", data)
		}
	}
	for _, data := range []string{`{"html":"<p>original</p>"}`, `https://example.com`, `{"url":"https://example.com"}`} {
		_, command, err := WebViewScriptUpdate(&WoxPreview{PreviewType: WoxPreviewTypeWebView, PreviewData: data})
		if command || err != nil {
			t.Fatal("legacy preview changed")
		}
	}
}
func TestScriptUpdatePreservesCachedHTMLAndUpdatesActions(t *testing.T) {
	original := WoxPreview{PreviewType: WoxPreviewTypeWebView, PreviewData: `{"html":"<p>original</p>"}`}
	manager, instance := newTestManagerWithCachedResult(Query{Id: "query", SessionId: "session", Type: QueryTypeInput, TriggerKeyword: "mo"}, QueryResult{Id: "row", Title: "Original", Preview: original})
	command := WoxPreview{PreviewType: WoxPreviewTypeWebView, PreviewData: `{"updateMode":"script","script":"tick()"}`}
	actions := []QueryResultAction{{Id: "stop", Name: "Stop", IsDefault: true}}
	result := manager.PolishUpdatableResult(context.Background(), instance, UpdatableResult{Id: "row", Preview: &command, Actions: &actions})
	if result.Preview.PreviewData != command.PreviewData {
		t.Fatal("command was lost")
	}
	cached, found := manager.findResultCacheById("row")
	if !found || cached.Result.Preview.PreviewData != original.PreviewData {
		t.Fatal("command replaced cached HTML")
	}
	if len(cached.Result.Actions) == 0 || cached.Result.Actions[0].Name != "Stop" {
		t.Fatal("ordinary fields lost")
	}
}

func TestScriptUpdateRejectsForeignOwnerAndMissingQuery(t *testing.T) {
	// Initialize the singleton once, then replace only for this synchronous API test.
	previous := GetPluginManager()
	defer func() { managerInstance = previous }()
	original := WoxPreview{PreviewType: WoxPreviewTypeWebView, PreviewData: `{"html":"<p>owned</p>"}`}
	manager, owner := newTestManagerWithCachedResult(Query{Id: "query", SessionId: "session", Type: QueryTypeInput, TriggerKeyword: "mo"}, QueryResult{Id: "row", Preview: original})
	managerInstance = manager
	command := WoxPreview{PreviewType: WoxPreviewTypeWebView, PreviewData: `{"updateMode":"script","script":"tick()"}`}
	update := UpdatableResult{Id: "row", Preview: &command}
	ctx := util.WithQueryIdContext(util.WithSessionContext(context.Background(), "session"), "query")
	foreign := &APIImpl{pluginInstance: &Instance{Metadata: Metadata{Id: "foreign"}}}
	if foreign.UpdateResult(ctx, update) {
		t.Fatal("foreign plugin accepted")
	}
	ownerAPI := &APIImpl{pluginInstance: owner}
	old := util.WithQueryIdContext(ctx, "obsolete")
	if ownerAPI.UpdateResult(old, update) {
		t.Fatal("old query accepted")
	}
	cached, _ := manager.findResultCacheById("row")
	cached.Result.Preview = WoxPreview{PreviewType: WoxPreviewTypeWebView, PreviewData: `{"url":"https://example.com"}`}
	if ownerAPI.UpdateResult(ctx, update) {
		t.Fatal("URL page accepted")
	}
}
