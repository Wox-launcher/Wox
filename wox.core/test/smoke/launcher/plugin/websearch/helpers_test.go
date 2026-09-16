//go:build wox_ui_smoke

package websearch

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	webSearchPluginID       = "c1e350a7-c521-4dc3-b4ff-509f720fde86"
	webSearchesTableAddID   = "plugin-settings-field-2-add"
	webSearchKeywordFieldID = "form-table-row-field-1"
	webSearchTitleFieldID   = "form-table-row-field-2"
	webSearchUrlsFieldID    = "form-table-row-field-3"
	webSearchUrlsTrailingID = "form-table-row-field-3-trailing"
	// Ungrouped editor fields are Icon, Keyword, Title, Urls, Browser, Disabled.
	webSearchDisabledFieldID   = "form-table-row-field-5"
	webSearchTitleErrorID      = "form-table-row-field-2-error"
	webSearchQueryVariableID   = "query-variable-0"
	webSearchQueryVariableMenu = "query-variable-picker"
	webSearchWebViewPreviewID  = "launcher.preview.webview"
)

func webSearchesRowEditID(index int) string {
	return fmt.Sprintf("plugin-settings-field-2-row-%d-edit", index)
}

func webSearchesRowDeleteID(index int) string {
	return fmt.Sprintf("plugin-settings-field-2-row-%d-delete", index)
}

func openWebSearchSettings(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	smoke.OpenInstalledPluginSettings(t, ctx, client, webSearchPluginID)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		add, found := automationdriver.Find(snapshot, webSearchesTableAddID)
		return found && add.Enabled
	}); err != nil {
		t.Fatalf("wait for Web Search settings: %v", err)
	}
}

func waitForWebSearchRowEditor(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, keywordFound := automationdriver.Find(snapshot, webSearchKeywordFieldID)
		_, titleFound := automationdriver.Find(snapshot, webSearchTitleFieldID)
		_, urlsFound := automationdriver.Find(snapshot, webSearchUrlsFieldID)
		_, saveFound := automationdriver.Find(snapshot, "form-table-row-save")
		return keywordFound && titleFound && urlsFound && saveFound
	}); err != nil {
		t.Fatalf("wait for Web Search row editor: %v", err)
	}
}

func setWebSearchRowText(t *testing.T, ctx context.Context, client *automationdriver.Client, id, value string) {
	t.Helper()
	if err := client.Perform(ctx, id, woxui.AccessibilityActionSetValue, value); err != nil {
		t.Fatalf("set %s: %v", id, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, id)
		return found && field.Value == value
	}); err != nil {
		t.Fatalf("wait for %s to become %q: %v", id, value, err)
	}
}

func ensureWebSearchRowNotDisabled(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, webSearchDisabledFieldID)
		return found
	}); err != nil {
		t.Fatalf("wait for Web Search disabled field: %v", err)
	}
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read Web Search disabled field: %v", err)
	}
	disabled, found := automationdriver.Find(snapshot, webSearchDisabledFieldID)
	if !found {
		t.Fatal("Web Search disabled field was not found")
	}
	if disabled.Checked {
		if err := client.Perform(ctx, webSearchDisabledFieldID, woxui.AccessibilityActionToggle, ""); err != nil {
			t.Fatalf("enable Web Search row: %v", err)
		}
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, webSearchDisabledFieldID)
		return found && !field.Checked
	}); err != nil {
		t.Fatalf("wait for Web Search row to stay enabled: %v", err)
	}
}

func saveWebSearchRow(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.Perform(ctx, "form-table-row-save", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("save Web Search row: %v", err)
	}
}

func addWebSearchRow(t *testing.T, ctx context.Context, client *automationdriver.Client, keyword, title, urls string) {
	t.Helper()
	openWebSearchSettings(t, ctx, client)
	if err := client.Perform(ctx, webSearchesTableAddID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("add Web Search row: %v", err)
	}
	waitForWebSearchRowEditor(t, ctx, client)
	setWebSearchRowText(t, ctx, client, webSearchKeywordFieldID, keyword)
	setWebSearchRowText(t, ctx, client, webSearchTitleFieldID, title)
	setWebSearchRowText(t, ctx, client, webSearchUrlsFieldID, urls)
	ensureWebSearchRowNotDisabled(t, ctx, client)
	saveWebSearchRow(t, ctx, client)
	waitForWebSearchEditorClosed(t, ctx, client)
	findWebSearchRow(t, ctx, client, keyword)
}

func confirmWebSearchRowPersisted(t *testing.T, ctx context.Context, client *automationdriver.Client, keyword, title, urls string) {
	t.Helper()
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close Web Search settings after save: %v", err)
	}
	openWebSearchSettings(t, ctx, client)
	index := findWebSearchRow(t, ctx, client, keyword)
	if err := client.Perform(ctx, webSearchesRowEditID(index), woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("inspect persisted Web Search %q: %v", keyword, err)
	}
	waitForWebSearchRowEditor(t, ctx, client)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		keywordField, keywordFound := automationdriver.Find(snapshot, webSearchKeywordFieldID)
		titleField, titleFound := automationdriver.Find(snapshot, webSearchTitleFieldID)
		urlsField, urlsFound := automationdriver.Find(snapshot, webSearchUrlsFieldID)
		disabled, disabledFound := automationdriver.Find(snapshot, webSearchDisabledFieldID)
		return keywordFound && keywordField.Value == keyword && titleFound && titleField.Value == title && urlsFound && urlsField.Value == urls && disabledFound && !disabled.Checked
	}); err != nil {
		t.Fatalf("confirm persisted Web Search %q: %v", keyword, err)
	}
	if err := client.Perform(ctx, "form-table-row-cancel", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("close Web Search inspection: %v", err)
	}
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close Web Search settings after inspection: %v", err)
	}
}

func waitForWebSearchEditorClosed(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		add, addFound := automationdriver.Find(snapshot, webSearchesTableAddID)
		_, editorFound := automationdriver.Find(snapshot, "form-table-row-save")
		return addFound && add.Enabled && !editorFound
	}); err != nil {
		t.Fatalf("wait for Web Search editor to close: %v", err)
	}
}

func findWebSearchRow(t *testing.T, ctx context.Context, client *automationdriver.Client, keyword string) int {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read Web Search rows: %v", err)
	}
	for index := 0; ; index++ {
		editID := webSearchesRowEditID(index)
		if _, exists := automationdriver.Find(snapshot, editID); !exists {
			break
		}
		if err := client.Perform(ctx, editID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("open Web Search row %d while looking for %q: %v", index, keyword, err)
		}
		waitForWebSearchRowEditor(t, ctx, client)
		current, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read Web Search row %d: %v", index, err)
		}
		field, found := automationdriver.Find(current, webSearchKeywordFieldID)
		if err := client.Perform(ctx, "form-table-row-cancel", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("close Web Search row %d: %v", index, err)
		}
		waitForWebSearchEditorClosed(t, ctx, client)
		if found && field.Value == keyword {
			return index
		}
	}
	t.Fatalf("Web Search row %q was not found", keyword)
	return -1
}

func deleteWebSearchRow(t *testing.T, client *automationdriver.Client, keyword string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), automationdriver.ActionTimeout)
	defer cancel()
	if err := client.Hide(ctx); err != nil {
		t.Errorf("hide active window before deleting Web Search %q: %v", keyword, err)
	}
	openWebSearchSettings(t, ctx, client)
	index, found := inspectWebSearchRow(t, ctx, client, keyword)
	if !found {
		if err := client.Hide(ctx); err != nil {
			t.Errorf("close Web Search settings after missing %q: %v", keyword, err)
		}
		return
	}
	deleteID := webSearchesRowDeleteID(index)
	if err := client.Perform(ctx, deleteID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Errorf("delete Web Search %q: %v", keyword, err)
		return
	}
	if err := client.Perform(ctx, deleteID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Errorf("confirm Web Search %q deletion: %v", keyword, err)
		return
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, editorFound := automationdriver.Find(snapshot, "form-table-row-save")
		_, deleteFound := automationdriver.Find(snapshot, deleteID)
		return !editorFound && !deleteFound
	}); err != nil {
		t.Errorf("wait for Web Search %q deletion: %v", keyword, err)
	}
	if err := client.Hide(ctx); err != nil {
		t.Errorf("close Web Search settings after deleting %q: %v", keyword, err)
	}
}

func inspectWebSearchRow(t *testing.T, ctx context.Context, client *automationdriver.Client, keyword string) (int, bool) {
	t.Helper()
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Errorf("read Web Search rows while searching for %q: %v", keyword, err)
		return 0, false
	}
	for index := 0; ; index++ {
		editID := webSearchesRowEditID(index)
		if _, exists := automationdriver.Find(snapshot, editID); !exists {
			return 0, false
		}
		if err := client.Perform(ctx, editID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Errorf("open Web Search row %d while deleting %q: %v", index, keyword, err)
			return 0, false
		}
		waitForWebSearchRowEditor(t, ctx, client)
		current, err := client.Snapshot(ctx)
		if err != nil {
			t.Errorf("read Web Search row %d while deleting %q: %v", index, keyword, err)
			return 0, false
		}
		field, found := automationdriver.Find(current, webSearchKeywordFieldID)
		if err := client.Perform(ctx, "form-table-row-cancel", woxui.AccessibilityActionActivate, ""); err != nil {
			t.Errorf("close Web Search row %d while deleting %q: %v", index, keyword, err)
			return 0, false
		}
		waitForWebSearchEditorClosed(t, ctx, client)
		if found && field.Value == keyword {
			return index, true
		}
	}
}

func insertWebSearchParameterFromPicker(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.Perform(ctx, webSearchUrlsTrailingID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("open query variable picker: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, menuFound := automationdriver.Find(snapshot, webSearchQueryVariableMenu)
		_, itemFound := automationdriver.Find(snapshot, webSearchQueryVariableID)
		return menuFound && itemFound
	}); err != nil {
		t.Fatalf("wait for query variable picker: %v", err)
	}
	if err := client.Perform(ctx, webSearchQueryVariableID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("insert input parameter: %v", err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, menuFound := automationdriver.Find(snapshot, webSearchQueryVariableMenu)
		urls, urlsFound := automationdriver.Find(snapshot, webSearchUrlsFieldID)
		return !menuFound && urlsFound && strings.Contains(urls.Value, "{wox:parameter?name=query}")
	}); err != nil {
		t.Fatalf("wait for inserted input parameter: %v", err)
	}
}

func typeWebSearchQuery(t *testing.T, ctx context.Context, client *automationdriver.Client, keyword string) {
	t.Helper()
	smoke.ShowLauncher(t, ctx, client)
	smoke.ReplaceLauncherQuery(t, ctx, client, "")
	if err := client.EnterText(ctx, keyword); err != nil {
		t.Fatalf("type Web Search keyword %q: %v", keyword, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		input, found := automationdriver.Find(snapshot, "launcher.query.input")
		_, shown := automationdriver.Find(snapshot, "launcher.query.completion")
		return found && input.Value == keyword && !shown
	}); err != nil {
		t.Fatalf("bare keyword %q must not show a parameter hint: %v", keyword, err)
	}
	if err := client.EnterText(ctx, " "); err != nil {
		t.Fatalf("type Web Search keyword separator: %v", err)
	}
}

func waitForWebSearchHint(t *testing.T, ctx context.Context, client *automationdriver.Client, query string, names ...string) {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		input, found := automationdriver.Find(snapshot, "launcher.query.input")
		hint, shown := automationdriver.Find(snapshot, "launcher.query.completion")
		if !found || input.Value != query || !shown {
			return false
		}
		for _, name := range names {
			if !strings.Contains(hint.Value, name) {
				return false
			}
		}
		return true
	})
	if err != nil {
		t.Fatalf("wait for Web Search hint %v after %q: %v", names, query, err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
}

func waitForWebSearchResult(t *testing.T, ctx context.Context, client *automationdriver.Client, query, title string) {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		input, found := automationdriver.Find(snapshot, "launcher.query.input")
		results, complete := automationdriver.Find(snapshot, "launcher.results")
		if !found || input.Value != query || !complete || results.Value != "complete" {
			return false
		}
		_, resultFound := smoke.FindLauncherResult(snapshot, title)
		return resultFound
	})
	if err != nil {
		t.Fatalf("wait for Web Search result %q: %v", title, err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
}

func skipWebSearchWebViewIfUnavailable(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		t.Skip("Web Search in-launcher WebView preview is only available on Windows and macOS")
	}
}

func webSearchPrimaryModifier() woxui.KeyModifiers {
	if runtime.GOOS == "darwin" {
		return woxui.KeyModifierMeta
	}
	return woxui.KeyModifierControl
}

// startWebSearchPreviewServer serves a local search page so the preview never needs the public internet.
func startWebSearchPreviewServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<!doctype html><html><body><h1>wox-websearch-preview</h1><p>%s</p></body></html>", html.EscapeString(r.URL.Query().Get("q")))
	}))
	t.Cleanup(server.Close)
	return server
}

func openWebSearchWebViewPreview(t *testing.T, ctx context.Context, client *automationdriver.Client, query, previewURL string) {
	t.Helper()
	if err := client.PressKey(ctx, woxui.KeyEnter, webSearchPrimaryModifier()); err != nil {
		t.Fatalf("open Web Search WebView preview: %v", err)
	}
	waitForWebSearchWebViewPreview(t, ctx, client, query, previewURL)
}

func waitForWebSearchWebViewPreview(t *testing.T, ctx context.Context, client *automationdriver.Client, query, previewURL string) {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		preview, previewFound := automationdriver.Find(snapshot, webSearchWebViewPreviewID)
		if previewFound && strings.HasPrefix(preview.Value, "error:") {
			return true
		}
		input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		return previewFound && preview.Value == previewURL && inputFound && input.Value == query && resultsFound && results.Value == "complete"
	})
	if err != nil {
		t.Fatalf("wait for Web Search WebView preview %q: %v", previewURL, err)
	}
	preview, found := automationdriver.Find(snapshot, webSearchWebViewPreviewID)
	if !found {
		t.Fatal("Web Search WebView preview node was missing after wait")
	}
	if strings.HasPrefix(preview.Value, "error:") {
		t.Fatalf("Web Search WebView preview failed: %s", preview.Value)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
}

func exitWebSearchWebViewPreview(t *testing.T, ctx context.Context, client *automationdriver.Client, query, title string) {
	t.Helper()
	if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
		t.Fatalf("leave Web Search WebView preview: %v", err)
	}
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, previewFound := automationdriver.Find(snapshot, webSearchWebViewPreviewID)
		input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		_, resultFound := smoke.FindLauncherResult(snapshot, title)
		return !previewFound && inputFound && input.Value == query && input.Focused && resultsFound && results.Value == "complete" && resultFound
	})
	if err != nil {
		t.Fatalf("wait for Web Search results after leaving WebView: %v", err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
}
