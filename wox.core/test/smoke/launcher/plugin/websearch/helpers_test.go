//go:build wox_ui_smoke

package websearch

import (
	"context"
	"fmt"
	"html"
	"math"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"wox/common"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	webSearchPluginID       = common.WebSearchPluginID
	webSearchesTableAddID   = "plugin-settings-field-2-add"
	webSearchKeywordFieldID = "form-table-row-field-1"
	webSearchTitleFieldID   = "form-table-row-field-2"
	webSearchUrlsFieldID    = "form-table-row-field-3"
	webSearchUrlsTrailingID = "form-table-row-field-3-trailing"
	// Column order is Icon, Keyword, Title, Urls, Browser, Disabled,
	// OpenInPrivate, IsFallback, OpenInWebView, WebViewWidth, WebViewHeight,
	// InjectCSS, UserAgent, KeepInBackground.
	webSearchDisabledFieldID         = "form-table-row-field-5"
	webSearchTitleErrorID            = "form-table-row-field-2-error"
	webSearchQueryVariableID         = "query-variable-0"
	webSearchQueryVariableMenu       = "query-variable-picker"
	webSearchWebViewPreviewID        = "launcher.preview.webview"
	webSearchWebViewGroupID          = "form-table-row-group-webview"
	webSearchOpenInWebViewFieldID    = "form-table-row-field-8"
	webSearchWebViewWidthFieldID     = "form-table-row-field-9"
	webSearchWebViewHeightFieldID    = "form-table-row-field-10"
	webSearchKeepInBackgroundFieldID = "form-table-row-field-13"
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

func fillWebSearchRowEditor(t *testing.T, ctx context.Context, client *automationdriver.Client, keyword, title, urls string) {
	t.Helper()
	waitForWebSearchRowEditor(t, ctx, client)
	setWebSearchRowText(t, ctx, client, webSearchKeywordFieldID, keyword)
	setWebSearchRowText(t, ctx, client, webSearchTitleFieldID, title)
	setWebSearchRowText(t, ctx, client, webSearchUrlsFieldID, urls)
	ensureWebSearchRowNotDisabled(t, ctx, client)
}

func addWebSearchRow(t *testing.T, ctx context.Context, client *automationdriver.Client, keyword, title, urls string) {
	t.Helper()
	openWebSearchSettings(t, ctx, client)
	if err := client.Perform(ctx, webSearchesTableAddID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("add Web Search row: %v", err)
	}
	fillWebSearchRowEditor(t, ctx, client, keyword, title, urls)
	saveWebSearchRow(t, ctx, client)
	waitForWebSearchEditorClosed(t, ctx, client)
	findWebSearchRow(t, ctx, client, keyword)
}

func addWebSearchRowWithPreviewSize(t *testing.T, ctx context.Context, client *automationdriver.Client, keyword, title, urls, width, height string) {
	t.Helper()
	openWebSearchSettings(t, ctx, client)
	if err := client.Perform(ctx, webSearchesTableAddID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("add Web Search row: %v", err)
	}
	fillWebSearchRowEditor(t, ctx, client, keyword, title, urls)
	setWebSearchWebViewSize(t, ctx, client, width, height)
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
	var err error
	if runtime.GOOS == "windows" {
		// WebView2 environment creation synchronously blocked the CI UI thread for
		// 9.89s on a cold profile, leaving no room in the ordinary 10s key budget.
		err = client.PressKeyWithTimeout(ctx, woxui.KeyEnter, webSearchPrimaryModifier(), 30*time.Second)
	} else {
		err = client.PressKey(ctx, woxui.KeyEnter, webSearchPrimaryModifier())
	}
	if err != nil {
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

func expandWebSearchWebViewGroup(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, webSearchWebViewGroupID)
		return found
	}); err != nil {
		t.Fatalf("wait for Web Search WebView group: %v", err)
	}
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read Web Search WebView group: %v", err)
	}
	group, found := automationdriver.Find(snapshot, webSearchWebViewGroupID)
	if !found {
		t.Fatal("Web Search WebView group was not found")
	}
	if !group.Expanded {
		if err := client.Perform(ctx, webSearchWebViewGroupID, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("expand Web Search WebView group: %v", err)
		}
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		header, headerFound := automationdriver.Find(snapshot, webSearchWebViewGroupID)
		_, widthFound := automationdriver.Find(snapshot, webSearchWebViewWidthFieldID)
		_, heightFound := automationdriver.Find(snapshot, webSearchWebViewHeightFieldID)
		return headerFound && header.Expanded && widthFound && heightFound
	}); err != nil {
		t.Fatalf("wait for Web Search WebView size fields: %v", err)
	}
}

func setWebSearchWebViewSize(t *testing.T, ctx context.Context, client *automationdriver.Client, width, height string) {
	t.Helper()
	expandWebSearchWebViewGroup(t, ctx, client)
	setWebSearchRowText(t, ctx, client, webSearchWebViewWidthFieldID, width)
	setWebSearchRowText(t, ctx, client, webSearchWebViewHeightFieldID, height)
}

func confirmWebSearchRowPreviewSizePersisted(t *testing.T, ctx context.Context, client *automationdriver.Client, keyword, title, urls, width, height string) {
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
	expandWebSearchWebViewGroup(t, ctx, client)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		keywordField, keywordFound := automationdriver.Find(snapshot, webSearchKeywordFieldID)
		titleField, titleFound := automationdriver.Find(snapshot, webSearchTitleFieldID)
		urlsField, urlsFound := automationdriver.Find(snapshot, webSearchUrlsFieldID)
		widthField, widthFound := automationdriver.Find(snapshot, webSearchWebViewWidthFieldID)
		heightField, heightFound := automationdriver.Find(snapshot, webSearchWebViewHeightFieldID)
		return keywordFound && keywordField.Value == keyword && titleFound && titleField.Value == title && urlsFound && urlsField.Value == urls &&
			widthFound && widthField.Value == width && heightFound && heightField.Value == height
	}); err != nil {
		t.Fatalf("confirm persisted Web Search preview size for %q: %v", keyword, err)
	}
	if err := client.Perform(ctx, "form-table-row-cancel", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("close Web Search inspection: %v", err)
	}
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close Web Search settings after inspection: %v", err)
	}
}

func readLauncherBounds(t *testing.T, ctx context.Context, client *automationdriver.Client) woxui.Rect {
	t.Helper()
	bounds, err := client.Bounds(ctx)
	if err != nil {
		t.Fatalf("read launcher bounds: %v", err)
	}
	return bounds
}

func waitForWebSearchPreviewBounds(t *testing.T, ctx context.Context, client *automationdriver.Client, width, height float32) {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var last woxui.Rect
	for {
		bounds, err := client.Bounds(ctx)
		if err != nil {
			t.Fatalf("read launcher bounds while waiting for preview size %.0fx%.0f: %v", width, height, err)
		}
		last = bounds
		if math.Abs(float64(bounds.Width-width)) <= 1 && math.Abs(float64(bounds.Height-height)) <= 1 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for Web Search preview size %.0fx%.0f: last bounds %+v; %v", width, height, last, ctx.Err())
		case <-ticker.C:
		}
	}
}

// setWebSearchCheckbox toggles a row option only when its current value differs.
func setWebSearchCheckbox(t *testing.T, ctx context.Context, client *automationdriver.Client, id string, checked bool) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, found := automationdriver.Find(snapshot, id)
		return found
	}); err != nil {
		t.Fatalf("wait for Web Search checkbox %s: %v", id, err)
	}
	snapshot, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("read Web Search checkbox %s: %v", id, err)
	}
	field, found := automationdriver.Find(snapshot, id)
	if !found {
		t.Fatalf("Web Search checkbox %s was not found", id)
	}
	if field.Checked == checked {
		return
	}
	if err := client.Perform(ctx, id, woxui.AccessibilityActionToggle, ""); err != nil {
		t.Fatalf("toggle Web Search checkbox %s: %v", id, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, id)
		return found && field.Checked == checked
	}); err != nil {
		t.Fatalf("wait for Web Search checkbox %s checked=%t: %v", id, checked, err)
	}
}

// addWebSearchAutomaticPreviewRow saves an opt-in WebView search through its settings editor.
func addWebSearchAutomaticPreviewRow(t *testing.T, ctx context.Context, client *automationdriver.Client, keyword, title, urls, width, height string, keepInBackground bool) {
	t.Helper()
	openWebSearchSettings(t, ctx, client)
	if err := client.Perform(ctx, webSearchesTableAddID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("add Web Search row: %v", err)
	}
	fillWebSearchRowEditor(t, ctx, client, keyword, title, urls)
	expandWebSearchWebViewGroup(t, ctx, client)
	setWebSearchCheckbox(t, ctx, client, webSearchOpenInWebViewFieldID, true)
	if width != "" || height != "" {
		setWebSearchWebViewSize(t, ctx, client, width, height)
	}
	setWebSearchCheckbox(t, ctx, client, webSearchKeepInBackgroundFieldID, keepInBackground)
	saveWebSearchRow(t, ctx, client)
	waitForWebSearchEditorClosed(t, ctx, client)
	findWebSearchRow(t, ctx, client, keyword)
}

// confirmWebSearchAutomaticPreviewPersisted reopens settings before exercising the saved options.
func confirmWebSearchAutomaticPreviewPersisted(t *testing.T, ctx context.Context, client *automationdriver.Client, keyword, title, urls, width, height string, keepInBackground bool) {
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
	expandWebSearchWebViewGroup(t, ctx, client)
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		keywordField, keywordFound := automationdriver.Find(snapshot, webSearchKeywordFieldID)
		titleField, titleFound := automationdriver.Find(snapshot, webSearchTitleFieldID)
		urlsField, urlsFound := automationdriver.Find(snapshot, webSearchUrlsFieldID)
		openField, openFound := automationdriver.Find(snapshot, webSearchOpenInWebViewFieldID)
		widthField, widthFound := automationdriver.Find(snapshot, webSearchWebViewWidthFieldID)
		heightField, heightFound := automationdriver.Find(snapshot, webSearchWebViewHeightFieldID)
		keepField, keepFound := automationdriver.Find(snapshot, webSearchKeepInBackgroundFieldID)
		return keywordFound && keywordField.Value == keyword && titleFound && titleField.Value == title && urlsFound && urlsField.Value == urls &&
			openFound && openField.Checked && widthFound && widthField.Value == width && heightFound && heightField.Value == height &&
			keepFound && keepField.Checked == keepInBackground
	}); err != nil {
		t.Fatalf("confirm persisted automatic Web Search preview for %q: %v", keyword, err)
	}
	if err := client.Perform(ctx, "form-table-row-cancel", woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("close Web Search inspection: %v", err)
	}
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close Web Search settings after inspection: %v", err)
	}
}

// setWebSearchKeepInBackground edits retention without replacing the search row.
func setWebSearchKeepInBackground(t *testing.T, ctx context.Context, client *automationdriver.Client, keyword string, keep bool) {
	t.Helper()
	openWebSearchSettings(t, ctx, client)
	index := findWebSearchRow(t, ctx, client, keyword)
	if err := client.Perform(ctx, webSearchesRowEditID(index), woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("edit Web Search %q: %v", keyword, err)
	}
	waitForWebSearchRowEditor(t, ctx, client)
	expandWebSearchWebViewGroup(t, ctx, client)
	setWebSearchCheckbox(t, ctx, client, webSearchKeepInBackgroundFieldID, keep)
	saveWebSearchRow(t, ctx, client)
	waitForWebSearchEditorClosed(t, ctx, client)
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close Web Search settings after changing background retention: %v", err)
	}
}

// startCountingWebSearchPreviewServer counts document loads so retention can be distinguished from a reload.
func startCountingWebSearchPreviewServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" {
			requests.Add(1)
		}
		// A recreated page must reach the server instead of an HTTP cache.
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<!doctype html><html><body><h1>wox-websearch-preview</h1><p>%s</p></body></html>", html.EscapeString(r.URL.Query().Get("q")))
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

// queryWebSearchAutomaticPreview starts a fresh parameterized query without invoking a preview action.
func queryWebSearchAutomaticPreview(t *testing.T, ctx context.Context, client *automationdriver.Client, keyword, query, previewURL string, requests *atomic.Int32) {
	t.Helper()
	before := requests.Load()
	typeWebSearchQuery(t, ctx, client, keyword)
	waitForWebSearchHint(t, ctx, client, keyword+" ", "query")
	if err := client.EnterText(ctx, "hello"); err != nil {
		t.Fatal(err)
	}
	waitForWebSearchAutomaticPreview(t, ctx, client, query, previewURL, requests, before+1)
}

// waitForWebSearchAutomaticPreview requires both query semantics and a real native document request.
func waitForWebSearchAutomaticPreview(t *testing.T, ctx context.Context, client *automationdriver.Client, query, previewURL string, requests *atomic.Int32, minimum int32) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var last string
	for {
		snapshot, err := client.Snapshot(ctx)
		if err == nil {
			preview, previewFound := automationdriver.Find(snapshot, webSearchWebViewPreviewID)
			input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
			results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
			if previewFound && strings.HasPrefix(preview.Value, "error:") {
				t.Fatalf("Web Search WebView preview failed: %s", preview.Value)
			}
			count := requests.Load()
			if previewFound && preview.Value == previewURL && inputFound && input.Value == query && input.Focused && resultsFound && results.Value == "complete" && count >= minimum {
				smoke.AssertNoDiagnostics(t, snapshot)
				return
			}
			last = fmt.Sprintf("preview=%t value=%q input=%t query=%q focused=%t results=%t status=%q count=%d minimum=%d", previewFound, preview.Value, inputFound, input.Value, input.Focused, resultsFound, results.Value, count, minimum)
		} else {
			last = err.Error()
		}
		if time.Now().After(deadline) {
			t.Fatalf("wait for automatic Web Search preview %q: %s", previewURL, last)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for automatic Web Search preview %q: %s; %v", previewURL, last, ctx.Err())
		case <-ticker.C:
		}
	}
}

// focusWebSearchAutomaticPreview verifies Enter transfers focus without navigating again.
func focusWebSearchAutomaticPreview(t *testing.T, ctx context.Context, client *automationdriver.Client, query, previewURL string, requests *atomic.Int32) {
	t.Helper()
	before := requests.Load()
	if err := client.PressKey(ctx, woxui.KeyEnter, 0); err != nil {
		t.Fatalf("focus Web Search preview: %v", err)
	}
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		preview, previewFound := automationdriver.Find(snapshot, webSearchWebViewPreviewID)
		input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
		return previewFound && preview.Value == previewURL && inputFound && input.Value == query && !input.Focused
	})
	if err != nil {
		t.Fatalf("wait for Web Search preview focus: %v", err)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
	assertWebSearchPreviewRetained(t, ctx, client, query, previewURL, requests, before)
}

// hideAndShowWebSearchLauncher crosses the actual native hide/show lifecycle.
func hideAndShowWebSearchLauncher(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("hide launcher: %v", err)
	}
	if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
		return state.Exists && !state.Visible
	}); err != nil {
		t.Fatalf("wait for hidden launcher: %v", err)
	}
	smoke.ShowLauncher(t, ctx, client)
}

// assertWebSearchPreviewRetained observes a bounded interval to catch asynchronous navigation after show or focus.
func assertWebSearchPreviewRetained(t *testing.T, ctx context.Context, client *automationdriver.Client, query, previewURL string, requests *atomic.Int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		if got := requests.Load(); got != want {
			t.Fatalf("retained Web Search preview reloaded: requests=%d, want %d", got, want)
		}
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read retained Web Search preview: %v", err)
		}
		preview, previewFound := automationdriver.Find(snapshot, webSearchWebViewPreviewID)
		input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
		if !previewFound || preview.Value != previewURL || !inputFound || input.Value != query {
			t.Fatalf("retained Web Search preview changed: preview=%t value=%q input=%t query=%q", previewFound, preview.Value, inputFound, input.Value)
		}
		if time.Now().After(deadline) {
			smoke.AssertNoDiagnostics(t, snapshot)
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("retained Web Search preview wait ended: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}
