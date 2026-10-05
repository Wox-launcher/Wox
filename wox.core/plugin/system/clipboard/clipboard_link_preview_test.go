package system

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"wox/plugin"
	"wox/util"
	"wox/util/clipboard"
)

func TestClipboardLinkURLRejectsUnsupportedTargets(t *testing.T) {
	blocked := []string{
		"https://user:pass@8.8.8.8/private",
		"file:///C:/Windows/win.ini",
		"javascript:alert(1)",
		"http:///missing-host",
	}
	for _, raw := range blocked {
		if err := validateClipboardLinkURL(raw); err == nil {
			t.Fatalf("validate %s succeeded", raw)
		}
	}

	allowed := []string{
		"https://github.com/org/repo",
		"http://127.0.0.1/secret",
		"http://192.168.1.20/router",
		"http://198.18.0.16/",
		"https://localhost/page",
		"https://8.8.8.8/dns",
	}
	for _, raw := range allowed {
		if err := validateClipboardLinkURL(raw); err != nil {
			t.Fatalf("validate %s: %v", raw, err)
		}
	}
}

func TestParseClipboardLinkHTMLPrefersOpenGraph(t *testing.T) {
	body := `<html><head>
		<title>Ignored</title>
		<meta content="Hello &amp; Wox" property="og:title">
		<meta property="og:description" content="A launcher&#39;s preview">
		<meta property="og:image" content="/cover.png">
		<meta name="twitter:title" content="Tweet">
		<meta name="description" content="Fallback">
	</head></html>`
	meta := parseClipboardLinkHTML(body)
	if meta.Title != "Hello & Wox" || meta.Description != "A launcher's preview" || meta.ImageURL != "/cover.png" {
		t.Fatalf("meta = %+v", meta)
	}
}

func TestParseClipboardLinkHTMLUsesTwitterAndTitleFallback(t *testing.T) {
	body := `<title> Page
		title </title><meta name="twitter:description" content="Shown">`
	meta := parseClipboardLinkHTML(body)
	if meta.Title != "Page title" || meta.Description != "Shown" || meta.ImageURL != "" {
		t.Fatalf("meta = %+v", meta)
	}
}

func TestParseClipboardLinkHTMLHandlesHTMLBoundaries(t *testing.T) {
	cases := []struct {
		name string
		body string
		want clipboardLinkMeta
	}{
		{name: "quoted delimiter", body: `<meta property="og:title" content="A > B">`, want: clipboardLinkMeta{Title: "A > B"}},
		{name: "unicode offsets", body: strings.Repeat("\u0130", 80) + `<title>Hello</title><meta property="og:description" content="World">`, want: clipboardLinkMeta{Title: "Hello", Description: "World"}},
		{name: "comment", body: `<!-- <meta property="og:title" content="Wrong"> --><title>Right</title>`, want: clipboardLinkMeta{Title: "Right"}},
		{name: "script", body: `<script>const tag = '<meta property="og:title" content="Wrong">';</script><title>Right</title>`, want: clipboardLinkMeta{Title: "Right"}},
		{name: "entity decoded once", body: `<meta property="og:title" content="&amp;amp;"><meta property="og:image" content="/image.png?a=1&amp;b=2">`, want: clipboardLinkMeta{Title: "&amp;", ImageURL: "/image.png?a=1&b=2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseClipboardLinkHTML(tc.body); got != tc.want {
				t.Fatalf("metadata = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestClipboardLinkPreviewFromHTMLAndDirectImage(t *testing.T) {
	pngBytes := testClipboardLinkPNG(t)

	var requested []string
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requested = append(requested, request.URL.String())
		if request.URL.String() != "https://example.com/cover.png" {
			t.Errorf("unexpected image request %s", request.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"image/png"}},
			Body:       io.NopCloser(bytes.NewReader(pngBytes)),
			Request:    request,
		}, nil
	})}

	page := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body: io.NopCloser(strings.NewReader(
			`<meta property="og:title" content="Wox"><meta property="og:description" content="Search"><meta property="og:image" content="/cover.png">`,
		)),
		Request: newClipboardLinkTestRequest("https://example.com/post"),
	}
	preview, err := clipboardLinkPreviewFromResponse(context.Background(), client, "https://example.com/post", page)
	if err != nil {
		t.Fatalf("preview from html: %v", err)
	}
	if preview.Title != "Wox" || preview.Description != "Search" || preview.DirectImage || len(preview.imagePNG) == 0 {
		t.Fatalf("preview = %+v image=%d", preview, len(preview.imagePNG))
	}
	if len(requested) != 1 {
		t.Fatalf("image requests = %#v", requested)
	}

	direct := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"image/png"}},
		Body:       io.NopCloser(bytes.NewReader(pngBytes)),
		Request:    newClipboardLinkTestRequest("https://example.com/shot.png"),
	}
	imagePreview, err := clipboardLinkPreviewFromResponse(context.Background(), client, "https://example.com/shot.png", direct)
	if err != nil {
		t.Fatalf("direct image: %v", err)
	}
	if !imagePreview.DirectImage || len(imagePreview.imagePNG) == 0 {
		t.Fatalf("direct preview = %+v", imagePreview)
	}
}

func TestClipboardLinkPreviewCacheAndResult(t *testing.T) {
	dir := t.TempDir()
	previousDir := clipboardLinkCacheDirectory
	clipboardLinkCacheDirectory = func() string { return dir }
	t.Cleanup(func() { clipboardLinkCacheDirectory = previousDir })

	normalized := "https://example.com/post"
	raw := "https://example.com/post"
	miss := buildClipboardLinkResultPreview(raw, normalized, clipboardLinkPreview{}, false)
	if miss.Type != plugin.WoxPreviewTypeMarkdown || miss.Data != formatClipboardLinkMarkdown(raw, normalized) || miss.SubTitle != "" {
		t.Fatalf("cache miss = %+v", miss)
	}

	preview := clipboardLinkPreview{
		Title:       "Wox",
		Description: "Search *everything*",
		DirectImage: false,
		imagePNG:    testClipboardLinkPNG(t),
	}
	if err := saveClipboardLinkPreview(normalized, &preview); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, ok := loadClipboardLinkPreview(context.Background(), normalized)
	if !ok || loaded.Title != "Wox" || loaded.Description != preview.Description || loaded.ImagePath == "" || loaded.DirectImage {
		t.Fatalf("loaded = %+v ok=%v", loaded, ok)
	}
	shown := buildClipboardLinkResultPreview(raw, normalized, loaded, true)
	if shown.Type != plugin.WoxPreviewTypeMarkdown || shown.SubTitle != "Wox" || !strings.Contains(shown.Data, "Search \\*everything\\*") {
		t.Fatalf("shown = %+v", shown)
	}
	imageMarkdown := "![](<" + filepath.ToSlash(loaded.ImagePath) + ">)"
	if !strings.Contains(shown.Data, imageMarkdown) {
		t.Fatalf("markdown image = %s, want %s", shown.Data, imageMarkdown)
	}

	direct := clipboardLinkPreview{DirectImage: true, ImagePath: loaded.ImagePath}
	imageResult := buildClipboardLinkResultPreview("https://example.com/shot.png", "https://example.com/shot.png", direct, true)
	if imageResult.Type != plugin.WoxPreviewTypeImage || !strings.HasPrefix(imageResult.Data, "absolute:") || imageResult.OverlayData != imageResult.Data {
		t.Fatalf("direct result = %+v", imageResult)
	}
}

func TestScheduleLinkPreviewFetchesVisibleLinkOnce(t *testing.T) {
	dir := t.TempDir()
	previousDir := clipboardLinkCacheDirectory
	previousFetch := fetchClipboardLinkPreview
	clipboardLinkCacheDirectory = func() string { return dir }
	var fetches atomic.Int32
	fetchClipboardLinkPreview = func(context.Context, string) (clipboardLinkPreview, error) {
		fetches.Add(1)
		return clipboardLinkPreview{Title: "Visible", imagePNG: testClipboardLinkPNG(t)}, nil
	}
	t.Cleanup(func() {
		clipboardLinkCacheDirectory = previousDir
		fetchClipboardLinkPreview = previousFetch
	})

	actions := []plugin.QueryResultAction{{Id: clipboardOpenLinkActionID, Name: "Open link"}, {Name: "Delete"}}
	initialPreview := plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeMarkdown, PreviewData: "old", DefaultHidden: true}
	api := &clipboardFavoritesTestAPI{updatable: &plugin.UpdatableResult{Id: "row", Preview: &initialPreview, Actions: &actions}}
	link := "https://example.com/visible"
	c := &ClipboardPlugin{api: api}
	query := plugin.Query{Id: "query", SessionId: "session"}
	c.beginClipboardLinkQuery(query)
	c.rememberClipboardLinks(query, []ClipboardRecord{{ID: "row", Type: "text", Content: link}})
	c.scheduleLinkPreviewFetch(context.Background(), link)
	c.backgroundTasks.Wait()
	if fetches.Load() != 1 || len(api.updates) != 1 || api.updates[0].Preview == nil || !strings.Contains(api.updates[0].Preview.PreviewData, "Visible") {
		t.Fatalf("fetches=%d updates=%d", fetches.Load(), len(api.updates))
	}
	if api.updates[0].SubTitle == nil || *api.updates[0].SubTitle != "Visible" {
		t.Fatalf("subtitle = %#v", api.updates[0].SubTitle)
	}
	if api.updates[0].Actions == nil || !clipboardActionsContain(*api.updates[0].Actions, clipboardCopyLinkTitleActionID) {
		t.Fatalf("actions = %#v", api.updates[0].Actions)
	}
	if clipboardResultActionIndex(plugin.QueryResult{Actions: *api.updates[0].Actions}, "i18n:plugin_clipboard_copy_link_title") != 1 {
		t.Fatalf("copy title was not placed after open link: %#v", api.updates[0].Actions)
	}
	if !api.updates[0].Preview.DefaultHidden {
		t.Fatal("preview lost its visibility default")
	}
	if (*api.updates[0].Actions)[1].ContextData["recordId"] != "row" {
		t.Fatal("copy title action lost its MRU context")
	}
	if util.GetContextQueryId(api.updateCtx) != query.Id || util.GetContextSessionId(api.updateCtx) != query.SessionId {
		t.Fatal("update lost its query scope")
	}
	if api.updates[0].Title != nil || api.updates[0].Icon != nil || api.updates[0].Tails != nil {
		t.Fatal("preview update included unrelated row fields")
	}
	c.scheduleLinkPreviewFetch(context.Background(), link)
	c.backgroundTasks.Wait()
	if fetches.Load() != 1 {
		t.Fatalf("fetches=%d, want 1", fetches.Load())
	}

	hiddenAPI := &clipboardFavoritesTestAPI{}
	hidden := &ClipboardPlugin{api: hiddenAPI}
	hiddenLink := "https://example.com/hidden"
	fetchClipboardLinkPreview = func(context.Context, string) (clipboardLinkPreview, error) {
		fetches.Add(1)
		return clipboardLinkPreview{Title: "Hidden"}, nil
	}
	hidden.scheduleLinkPreviewFetch(context.Background(), hiddenLink)
	hidden.backgroundTasks.Wait()
	if len(hiddenAPI.updates) != 0 {
		t.Fatalf("hidden updates = %d", len(hiddenAPI.updates))
	}
	if _, ok := loadClipboardLinkPreview(context.Background(), hiddenLink); !ok {
		t.Fatal("hidden preview was not cached")
	}
}

func TestClipboardLinkPreviewLogRedactsQuery(t *testing.T) {
	raw := "https://example.com/reset?token=secret"
	message := clipboardLinkPreviewLog("skipped", raw, &url.Error{Op: "Get", URL: raw, Err: errors.New("dial tcp: connection refused")})
	if strings.Contains(message, "token=secret") || strings.Contains(message, "reset") {
		t.Fatalf("log leaked url: %s", message)
	}
	plain := clipboardLinkPreviewLog("skipped", raw, errors.New("Get \""+raw+"\": timeout"))
	if strings.Contains(plain, "token=secret") {
		t.Fatalf("log leaked url: %s", plain)
	}
}

func TestClipboardLinkRecordUsesCachedPreview(t *testing.T) {
	dir := t.TempDir()
	previousDir := clipboardLinkCacheDirectory
	clipboardLinkCacheDirectory = func() string { return dir }
	t.Cleanup(func() { clipboardLinkCacheDirectory = previousDir })

	normalized := "https://example.com/record"
	if err := saveClipboardLinkPreview(normalized, &clipboardLinkPreview{
		Title:       "Record title",
		Description: "Record description",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}
	api := &imagePasteFailureAPI{}
	c := &ClipboardPlugin{api: api}
	result := c.convertTextRecord(context.Background(), ClipboardRecord{
		ID:      "record-1",
		Type:    string(clipboard.ClipboardTypeText),
		Content: normalized,
	}, plugin.Query{Env: plugin.QueryEnv{ActiveWindowTitle: "Test window", ActiveWindowPid: 1234}})

	if result.SubTitle != "Record title" || result.Preview.PreviewType != plugin.WoxPreviewTypeMarkdown || !strings.Contains(result.Preview.PreviewData, "Record description") {
		t.Fatalf("result title=%q preview=%+v", result.SubTitle, result.Preview)
	}
	if !clipboardResultHasAction(result, "i18n:plugin_clipboard_copy_link_title") || !clipboardResultHasAction(result, "i18n:plugin_clipboard_copy_link_description") {
		t.Fatalf("missing copy title or description action: %+v", result.Actions)
	}
}

func TestClipboardLinkCopyActionsKeepTranslatedOrderAndIdentity(t *testing.T) {
	c := &ClipboardPlugin{}
	actions := []plugin.QueryResultAction{
		{Id: "copy", Name: "Copy"},
		{Id: clipboardOpenLinkActionID, Name: "打开链接"},
		{Id: "delete", Name: "Delete"},
	}
	shown := clipboardLinkResultPreview{Title: "Page", Description: "Description"}
	updated := c.insertClipboardLinkCopyActions(actions, shown, "row")
	if len(updated) != 5 || updated[2].Id != clipboardCopyLinkTitleActionID || updated[3].Id != clipboardCopyLinkDescriptionActionID {
		t.Fatalf("copy actions are not next to the translated open action: %+v", updated)
	}
	updated[2].Name = "复制标题"
	updated[3].Name = "复制描述"
	if repeated := c.insertClipboardLinkCopyActions(updated, shown, "row"); len(repeated) != len(updated) {
		t.Fatalf("translated actions were duplicated: %+v", repeated)
	}
}

func TestScheduleLinkPreviewDeduplicatesConcurrentCaptures(t *testing.T) {
	dir := t.TempDir()
	previousDir, previousFetch := clipboardLinkCacheDirectory, fetchClipboardLinkPreview
	clipboardLinkCacheDirectory = func() string { return dir }
	var fetches atomic.Int32
	fetchClipboardLinkPreview = func(_ context.Context, raw string) (clipboardLinkPreview, error) {
		fetches.Add(1)
		return clipboardLinkPreview{Title: raw}, nil
	}
	t.Cleanup(func() {
		clipboardLinkCacheDirectory, fetchClipboardLinkPreview = previousDir, previousFetch
	})
	c := &ClipboardPlugin{api: &clipboardFavoritesTestAPI{}}
	start := make(chan struct{})
	var producers sync.WaitGroup
	for i := 0; i < 20; i++ {
		producers.Add(1)
		go func(index int) {
			defer producers.Done()
			<-start
			link := "https://example.com/one"
			if index%2 != 0 {
				link = "https://example.com/two"
			}
			c.scheduleLinkPreviewFetch(context.Background(), link)
		}(i)
	}
	close(start)
	producers.Wait()
	c.backgroundTasks.Wait()
	if fetches.Load() != 2 {
		t.Fatalf("fetches = %d, want one per URL", fetches.Load())
	}
}

type clipboardLinkRecentTestDB struct {
	clipboardQueryTestDB
}

func (clipboardLinkRecentTestDB) GetRecent(context.Context, int, int) ([]ClipboardRecord, error) {
	return []ClipboardRecord{
		{ID: "row", Type: "text", Content: "https://example.com/new", Timestamp: 2},
		{ID: "older", Type: "text", Content: "Older", Timestamp: 1},
	}, nil
}

func TestClipboardLinkPreviewDoesNotPatchSequentialPaste(t *testing.T) {
	dir := t.TempDir()
	previousDir := clipboardLinkCacheDirectory
	clipboardLinkCacheDirectory = func() string { return dir }
	t.Cleanup(func() { clipboardLinkCacheDirectory = previousDir })
	api := &clipboardFavoritesTestAPI{}
	favicons := util.NewHashMap[string, bool]()
	favicons.Store("example.com", true)
	c := &ClipboardPlugin{api: api, db: clipboardLinkRecentTestDB{}, faviconFetchAttempted: favicons}
	ctx := context.Background()
	c.Query(ctx, plugin.Query{Id: "links", SessionId: "session"})
	response := c.Query(ctx, plugin.Query{Id: "paste", SessionId: "session", Command: clipboardPasteCommand})
	if len(response.Results) != 1 {
		t.Fatalf("paste results = %d, want one", len(response.Results))
	}
	row := response.Results[0]
	if row.Preview.PreviewData != "" {
		t.Fatal("sequential paste exposed a preview")
	}
	api.updatable = &plugin.UpdatableResult{Id: row.Id, Preview: &row.Preview, SubTitle: &row.SubTitle, Actions: &row.Actions}
	if err := saveClipboardLinkPreview("https://example.com/new", &clipboardLinkPreview{Title: "Page title"}); err != nil {
		t.Fatal(err)
	}
	c.publishClipboardLinkPreview(ctx, "https://example.com/new")
	if len(api.updates) != 0 {
		t.Fatal("pending preview patched the sequential paste result")
	}
}

func TestClipboardLinksIgnoreObsoleteQueryResponse(t *testing.T) {
	c := &ClipboardPlugin{}
	old := plugin.Query{Id: "old", SessionId: "session"}
	current := plugin.Query{Id: "current", SessionId: "session"}
	c.beginClipboardLinkQuery(old)
	c.beginClipboardLinkQuery(current)
	c.rememberClipboardLinks(old, []ClipboardRecord{{ID: "row", Type: "text", Content: "https://example.com/old"}})
	if len(c.visibleClipboardLinks("https://example.com/old")) != 0 {
		t.Fatal("obsolete response restored pending rows")
	}
	c.rememberClipboardLinks(current, []ClipboardRecord{{ID: "row", Type: "text", Content: "https://example.com/current"}})
	rows := c.visibleClipboardLinks("https://example.com/current")
	if len(rows) != 1 || rows[0].QueryID != current.Id || rows[0].SessionID != current.SessionId {
		t.Fatalf("current rows = %+v", rows)
	}
}

// testClipboardLinkPNG encodes a small valid image for cache and HTTP fixtures.
func testClipboardLinkPNG(t *testing.T) []byte {
	t.Helper()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewNRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return encoded.Bytes()
}

// newClipboardLinkTestRequest creates the final response URL used by HTTP fixtures.
func newClipboardLinkTestRequest(rawURL string) *http.Request {
	request, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		panic(err)
	}
	return request
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}
