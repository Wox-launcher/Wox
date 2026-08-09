package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestWebViewPreviewRemainsVisibleBesideActionPanel(t *testing.T) {
	bounds, visible := webViewPreviewVisibleBounds(woxui.Rect{X: 300, Y: 100, Width: 900, Height: 600}, 850)
	if !visible {
		t.Fatal("WebView should remain visible beside the action panel")
	}
	if bounds.Width != 550 {
		t.Fatalf("WebView width = %v, want 550", bounds.Width)
	}
}

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
