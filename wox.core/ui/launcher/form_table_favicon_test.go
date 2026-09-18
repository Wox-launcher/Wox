package launcher

import (
	"context"
	"errors"
	"testing"
	"time"
	"wox/common"
	"wox/ui/contract"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

type faviconTestServices struct {
	contract.Services
	err   error
	ready chan struct{}
}

func (s *faviconTestServices) FetchWebsiteIcon(context.Context, string, string) (common.WoxImage, error) {
	<-s.ready
	return common.WoxImage{ImageType: common.WoxImageTypeBase64, ImageData: "data:image/png;base64,fixture"}, s.err
}

func TestFormTableFaviconShowsURLHint(t *testing.T) {
	app := trayQueryEditorTestApp(t)
	app.translations = map[string]string{
		"ui_image_editor_url_hint": "Paste an image URL or a website URL.",
	}
	app.openFormTableFavicon(0)
	if !formTableFaviconContainsText(app.buildFormTableFavicon(app.settingsTableEditor.favicon, woxcomponent.ControlTheme{}, 800, 600), "Paste an image URL or a website URL.") {
		t.Fatal("idle dialog must explain image and website URLs")
	}
	app.settingsTableEditor.favicon.error = "bad url"
	if !formTableFaviconContainsText(app.buildFormTableFavicon(app.settingsTableEditor.favicon, woxcomponent.ControlTheme{}, 800, 600), "bad url") {
		t.Fatal("errors must replace the URL hint")
	}
}

func TestFormTableURLFetchError(t *testing.T) {
	app := trayQueryEditorTestApp(t)
	app.translations = map[string]string{
		"ui_image_editor_url_failed":      "generic-fail",
		"ui_image_editor_url_unavailable": "expired-or-private",
	}
	if got := formTableURLFetchError(app, errors.New("network down")); got != "generic-fail" {
		t.Fatalf("generic error = %q", got)
	}
	if got := formTableURLFetchError(app, &util.HTTPStatusError{StatusCode: 404, URL: "https://example.com/a.png"}); got != "expired-or-private" {
		t.Fatalf("404 error = %q", got)
	}
}

func TestNormalizeFaviconURL(t *testing.T) {
	for input, want := range map[string]string{
		" example.com/path ":          "https://example.com/path",
		"http://localhost:8080/path":  "http://localhost:8080/path",
		"https://example.com?q=hello": "https://example.com?q=hello",
		"https://private-user-images.githubusercontent.com/1/a.png?jwt=a.b.c": "https://private-user-images.githubusercontent.com/1/a.png?jwt=a.b.c",
		"": "", "https://": "", "file:///tmp/icon": "", "javascript:alert(1)": "",
		"https://user:password@example.com": "", "https://bad host": "",
	} {
		if got := normalizeFaviconURL(input); got != want {
			t.Errorf("normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFormTableFaviconLifecycle(t *testing.T) {
	for _, outcome := range []string{"success", "failure", "dismissed", "reopened", "different-row"} {
		t.Run(outcome, func(t *testing.T) {
			app := trayQueryEditorTestApp(t)
			app.lifecycleCtx = context.Background()
			service := &faviconTestServices{ready: make(chan struct{})}
			if outcome == "failure" {
				service.err = errors.New("network unavailable")
			}
			app.services = service
			callbacks := make(chan func(), 1)
			state := app.settingsTableEditor
			original := state.rowForm.values["Icon"]
			app.openFormTableFavicon(1)
			if state.favicon != nil {
				t.Fatal("opened for a non-image field")
			}
			app.openFormTableFavicon(0)
			app.fetchFormTableFavicon()
			if state.favicon.error == "" || state.favicon.loading {
				t.Fatal("invalid URL must stay editable")
			}
			state.favicon.url = "example.com"
			if app.onFormTableTextInput(woxui.TextInputEvent{}) {
				t.Fatal("dialog text must reach its native field")
			}
			app.fetchFormTableFavicon()
			if !state.favicon.loading {
				t.Fatal("missing loading state")
			}
			app.uiCall = func(fn func()) error { callbacks <- fn; return nil }
			close(service.ready)
			select {
			case apply := <-callbacks:
				app.uiCall = nil
				switch outcome {
				case "dismissed":
					app.onFormTableKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true})
				case "reopened":
					app.closeFormTableFavicon()
					app.openFormTableFavicon(0)
				case "different-row":
					row := *state.rowForm
					state.rowForm = &row
				}
				apply()
			case <-time.After(5 * time.Second):
				t.Fatal("favicon request did not complete")
			}
			if outcome == "success" {
				if state.favicon != nil || state.rowForm.values["Icon"] == original {
					t.Fatal("downloaded icon was not applied")
				}
			} else {
				if state.rowForm.values["Icon"] != original {
					t.Fatal("failed or stale result changed the icon")
				}
				if outcome == "failure" && (state.favicon.loading || state.favicon.error == "") {
					t.Fatal("failure must allow retry and show an error")
				}
			}
		})
	}
}

func formTableFaviconContainsText(widget woxwidget.Widget, want string) bool {
	stateful, ok := widget.(woxwidget.Stateful)
	if !ok {
		return false
	}
	props, ok := stateful.Widget.(woxcomponent.DialogProps)
	if !ok {
		return false
	}
	flex, ok := props.Child.(woxwidget.Flex)
	if !ok {
		return false
	}
	for _, child := range flex.Children {
		switch node := child.(type) {
		case woxwidget.Text:
			if node.Value == want {
				return true
			}
		case woxwidget.TextBlock:
			if node.Value == want {
				return true
			}
		}
	}
	return false
}
