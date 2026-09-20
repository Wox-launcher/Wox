package launcher

import (
	"strings"
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWebViewPreviewURLChanged(t *testing.T) {
	if webViewPreviewURLChanged("", `{"url":"https://example.com"}`) {
		t.Fatal("first WebView activation must not reset an uninitialized native instance")
	}
	if webViewPreviewURLChanged(`{"url":"https://example.com","injectCss":"a"}`, `{"url":"https://example.com","injectCss":"b"}`) {
		t.Fatal("CSS-only changes must not reset the complete native instance")
	}
	if !webViewPreviewURLChanged(`{"url":"https://example.com/old"}`, `{"url":"https://example.com/new"}`) {
		t.Fatal("changed WebView URL must reset the native instance")
	}
}

func TestActivateWebViewPreviewReportsURLChange(t *testing.T) {
	app := &App{webViewPreviewData: `{"url":"https://example.com/old"}`, webViewPreviewError: "stale error"}
	if !app.activateWebViewPreview(`{"url":"https://example.com/new"}`) {
		t.Fatal("URL replacement was not reported")
	}
	if app.webViewPreviewError != "" {
		t.Fatalf("WebView error was not cleared: %q", app.webViewPreviewError)
	}
}

func TestWebViewPreviewContentPreservesUserAgent(t *testing.T) {
	data, err := decodeWebViewPreview(`{"url":"https://example.com","userAgent":"ExampleBrowser/1.0"}`)
	if err != nil {
		t.Fatalf("decode WebView preview: %v", err)
	}
	if content := data.content(); content.UserAgent != "ExampleBrowser/1.0" {
		t.Fatalf("WebView User-Agent = %q", content.UserAgent)
	}
}

func TestBuildWebViewPreviewExposesAutomationStatus(t *testing.T) {
	const previewURL = "https://example.com/search?q=wox"
	payload := `{"url":"` + previewURL + `"}`
	app := &App{}

	loading, ok := app.buildWebViewPreview(payload, uiPalette{}, 120, 80).(woxwidget.Semantics)
	if !ok || loading.AutomationID != webViewPreviewAutomationID || loading.Value != "loading" || loading.LiveRegion != woxui.AccessibilityLiveRegionPolite {
		t.Fatalf("loading preview semantics = %#v", loading)
	}

	app.webViewPreviewData = payload
	ready, ok := app.buildWebViewPreview(payload, uiPalette{}, 120, 80).(woxwidget.Semantics)
	if !ok || ready.AutomationID != webViewPreviewAutomationID || ready.Value != previewURL {
		t.Fatalf("ready preview semantics = %#v", ready)
	}

	app.webViewPreviewError = "webview missing"
	failed, ok := app.buildWebViewPreview(payload, uiPalette{}, 120, 80).(woxwidget.Semantics)
	if !ok || failed.AutomationID != webViewPreviewAutomationID || !strings.HasPrefix(failed.Value, "error:") || !strings.Contains(failed.Value, "webview missing") {
		t.Fatalf("error preview semantics = %#v", failed)
	}

	invalid, ok := app.buildWebViewPreview("{", uiPalette{}, 120, 80).(woxwidget.Semantics)
	if !ok || invalid.AutomationID != webViewPreviewAutomationID || !strings.HasPrefix(invalid.Value, "error:") {
		t.Fatalf("invalid preview semantics = %#v", invalid)
	}
}

func TestWebViewURLFormValidation(t *testing.T) {
	definitions := []formDefinition{{Value: formDefinitionValue{
		Key: "Url", Validators: []formValidator{{Type: "is_url"}},
	}}}
	if errors := validateFormFieldErrors(definitions, map[string]string{"Url": "example.com"}); errors["Url"] != "i18n:ui_validator_must_be_url" {
		t.Fatalf("invalid URL error = %q", errors["Url"])
	}
	if errors := validateFormFieldErrors(definitions, map[string]string{"Url": "https://example.com"}); errors != nil {
		t.Fatalf("absolute URL validation errors = %#v", errors)
	}
}

func TestSyncWebViewActionHotkeyNilSafe(t *testing.T) {
	var app *App
	app.syncWebViewActionHotkey()
	(&App{}).syncWebViewActionHotkey()
}

// TestHTMLPreviewTransitionsKeepNativeCache leaves HTML expiry to the shared runtime.
func TestHTMLPreviewTransitionsKeepNativeCache(t *testing.T) {
	const html = `{"html":"<p>preview</p>","cacheDisabled":true}`
	const url = `{"url":"https://example.com"}`
	if webViewPreviewURLChanged(html, url) || webViewPreviewURLChanged(url, html) {
		t.Fatal("switching to or from HTML must not reset native URL and HTML sessions")
	}
	app := &App{webViewPreviewData: html}
	if !app.hasCacheableWebViewPreviewLocked() {
		t.Fatal("HTML secondary window must survive hide until its temporary session expires")
	}
}
