package launcher

import (
	"testing"
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
