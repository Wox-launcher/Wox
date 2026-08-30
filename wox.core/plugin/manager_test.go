package plugin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"wox/common"
	"wox/setting"
	"wox/util"
)

func TestLargeMediaPreviewBypassesRemoteWrapping(t *testing.T) {
	previewData := strings.Repeat("x", previewDataMaxSize+1)
	preview := WoxPreview{PreviewType: WoxPreviewTypeMedia, PreviewData: previewData}
	assert.False(t, shouldWrapRemotePreview(preview))
	assert.True(t, shouldWrapRemotePreview(WoxPreview{PreviewType: WoxPreviewTypeText, PreviewData: previewData}))

	result := (&Manager{}).buildResultUI(&QueryResultCache{
		Result: QueryResult{Id: "media-result", Preview: preview},
		Query:  Query{SessionId: "session"},
	}, "query")

	assert.Equal(t, WoxPreviewTypeMedia, result.Preview.PreviewType)
	assert.Equal(t, previewData, result.Preview.PreviewData)
}

func Test_QueryShortcut(t *testing.T) {
	shortcuts := []setting.QueryShortcut{
		{
			Shortcut: "wi",
			Query:    "wpm install",
		},
		{
			Shortcut: "wix",
			Query:    "wpm install {0} x {1}",
		},
	}

	query := GetPluginManager().expandQueryShortcut(util.NewTraceContext(), "wi 1 2", shortcuts)
	assert.Equal(t, "wpm install 1 2", query)

	query = GetPluginManager().expandQueryShortcut(util.NewTraceContext(), "wi wi 1 2", shortcuts)
	assert.Equal(t, "wpm install wi 1 2", query)

	query = GetPluginManager().expandQueryShortcut(util.NewTraceContext(), "wix 1 2", shortcuts)
	assert.Equal(t, "wpm install 1 x 2", query)

	query = GetPluginManager().expandQueryShortcut(util.NewTraceContext(), "wix 1 2 3 4", shortcuts)
	assert.Equal(t, "wpm install 1 x 2 3 4", query)

	query = GetPluginManager().expandQueryShortcut(util.NewTraceContext(), "wix 1", shortcuts)
	assert.Equal(t, "wpm install 1 x {1}", query)
}

func TestPolishUpdatableResultClearsPreviewForGlobalQuery(t *testing.T) {
	manager, pluginInstance := newTestManagerWithCachedResult(Query{
		Id:        "query-global",
		SessionId: "session",
		Type:      QueryTypeInput,
		RawQuery:  "pause",
		Search:    "pause",
	}, QueryResult{
		Id:    "result-global",
		Title: "Song",
	})
	preview := WoxPreview{
		PreviewType: WoxPreviewTypeImage,
		PreviewData: "base64:cover",
	}

	result := manager.PolishUpdatableResult(context.Background(), pluginInstance, UpdatableResult{
		Id:      "result-global",
		Preview: &preview,
	})

	assert.NotNil(t, result.Preview)
	assert.True(t, result.Preview.IsEmpty())

	cachedResult, found := manager.findResultCacheById("result-global")
	assert.True(t, found)
	assert.True(t, cachedResult.Result.Preview.IsEmpty())
}

func TestPolishUpdatableResultKeepsPreviewForTriggeredQuery(t *testing.T) {
	manager, pluginInstance := newTestManagerWithCachedResult(Query{
		Id:             "query-triggered",
		SessionId:      "session",
		Type:           QueryTypeInput,
		RawQuery:       "media",
		TriggerKeyword: "media",
	}, QueryResult{
		Id:    "result-triggered",
		Title: "Song",
	})
	preview := WoxPreview{
		PreviewType: WoxPreviewTypeImage,
		PreviewData: "base64:cover",
	}

	result := manager.PolishUpdatableResult(context.Background(), pluginInstance, UpdatableResult{
		Id:      "result-triggered",
		Preview: &preview,
	})

	assert.NotNil(t, result.Preview)
	assert.Equal(t, "base64:cover", result.Preview.PreviewData)
}

func TestNormalizeToolbarMsgUsesPluginIconWhenMsgIconMissing(t *testing.T) {
	manager := &Manager{}
	pluginIcon := common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><path d="M0 0h1v1H0z"/></svg>`)
	pluginInstance := &Instance{
		Metadata: Metadata{
			Icon: pluginIcon.String(),
		},
	}

	normalized := manager.normalizeToolbarMsg(context.Background(), pluginInstance, ToolbarMsg{Id: "status", Title: "working"})

	assert.Equal(t, pluginIcon, normalized.Icon)
}

func TestAppendActionEnglishAliasPreservesCustomAliases(t *testing.T) {
	pluginInstance := &Instance{Metadata: Metadata{I18n: map[string]map[string]string{
		"en_US": {"copy_path": "Copy Path"},
	}}}
	aliases := appendActionEnglishAlias(context.Background(), pluginInstance, "i18n:copy_path", []string{"文件地址"})
	assert.Equal(t, []string{"文件地址", "Copy Path"}, aliases)
	assert.Equal(t, aliases, appendActionEnglishAlias(context.Background(), pluginInstance, "i18n:copy_path", aliases))
}

func TestOpenPluginSettingActionIncludesEnglishAlias(t *testing.T) {
	pluginInstance := &Instance{Metadata: Metadata{Name: "System Command"}}
	action := (&Manager{}).newOpenPluginSettingAction(context.Background(), pluginInstance)
	assert.Equal(t, []string{"Open System Command settings"}, action.SearchAliases)
}

func TestConvertActionIconsReusesConvertedSource(t *testing.T) {
	icon := common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"><path d="M0 0h1v1H0z"/></svg>`)
	cache := make(map[common.WoxImage]common.WoxImage)
	first := []QueryResultAction{{Icon: icon}, {Icon: icon}}
	second := []QueryResultAction{{Icon: icon}}

	if converted := convertActionIcons(context.Background(), first, "", cache); converted != 1 {
		t.Fatalf("first conversion count = %d, want 1", converted)
	}
	if converted := convertActionIcons(context.Background(), second, "", cache); converted != 0 {
		t.Fatalf("reused conversion count = %d, want 0", converted)
	}
	if first[0].Icon != icon || first[1].Icon != icon || second[0].Icon != icon {
		t.Fatal("reused action icon changed converted value")
	}
}

func TestConvertResultIconDefersRemoteURLWithoutRequest(t *testing.T) {
	initPluginImageTestLocation(t)
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requestCount++
	}))
	defer server.Close()
	manager := &Manager{lazyResultIcons: util.NewHashMap[string, *lazyResultIconEntry]()}
	query := Query{Id: "query-remote-icon", SessionId: "session-remote-icon"}

	converted := manager.convertResultIcon(context.Background(), nil, query, QueryLayout{}, "result-remote-icon", "Remote", common.NewWoxImageUrl(server.URL+"/icon.png"))

	if requestCount != 0 {
		t.Fatalf("result polish made %d synchronous remote icon requests", requestCount)
	}
	payload, err := common.ParseWoxLazyLoadImagePayload(converted)
	if err != nil {
		t.Fatalf("parse registered lazy icon: %v", err)
	}
	if payload.Token == "" {
		t.Fatal("manager did not register a lazy remote icon token")
	}
	if _, found := manager.lazyResultIcons.Load(payload.Token); !found {
		t.Fatal("registered lazy remote icon token is missing from manager cache")
	}
}

func TestLoadLazyResultIconFallsBackWhenRemoteURLFails(t *testing.T) {
	initPluginImageTestLocation(t)
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		requestCount++
		http.Error(response, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	query := Query{Id: "query-remote-icon", SessionId: "session-remote-icon"}
	result := QueryResult{Id: "result-remote-icon"}
	manager, _ := newTestManagerWithCachedResult(query, result)
	manager.lazyResultIcons = util.NewHashMap[string, *lazyResultIconEntry]()
	token := "remote-icon-token"
	source := common.NewWoxImageUrl(server.URL + "/icon.png")
	lazyIcon := common.NewWoxImageLazyLoad(token, source.Hash(), common.ImageThumbnailPlaceholderIcon, common.ResultListIconSize)
	cachedResult, found := manager.findResultCacheInSession(query.SessionId, query.Id, result.Id)
	if !found {
		t.Fatal("expected cached result")
	}
	cachedResult.Result.Icon = lazyIcon
	manager.lazyResultIcons.Store(token, &lazyResultIconEntry{
		SessionId: query.SessionId, QueryId: query.Id, ResultId: result.Id,
		OriginalIcon: source, TargetSize: common.ResultListIconSize,
	})

	loaded, err := manager.LoadLazyResultIcon(context.Background(), token)
	if err != nil {
		t.Fatalf("load lazy remote icon: %v", err)
	}
	if loaded != common.ImageThumbnailPlaceholderIcon {
		t.Fatalf("failed remote URL should resolve to placeholder, got %+v", loaded)
	}
	if cachedResult.Result.Icon != common.ImageThumbnailPlaceholderIcon {
		t.Fatalf("cached failed remote URL should be placeholder, got %+v", cachedResult.Result.Icon)
	}
	if requestCount != 1 {
		t.Fatalf("failed remote URL made %d requests, want 1", requestCount)
	}
}

func initPluginImageTestLocation(t *testing.T) {
	t.Helper()
	dataDir := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, dataDir)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(dataDir, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatalf("init test location: %v", err)
	}
	common.ClearConvertIconPathExistenceCache()
}

func newTestManagerWithCachedResult(query Query, result QueryResult) (*Manager, *Instance) {
	manager := &Manager{
		sessionQueryResultCache: util.NewHashMap[string, *util.HashMap[string, *QueryResultSet]](),
	}
	pluginInstance := &Instance{
		Metadata: Metadata{
			Id:              "test-plugin",
			TriggerKeywords: []string{"*", "media"},
		},
	}
	sessionQueries := util.NewHashMap[string, *QueryResultSet]()
	resultSet := newQueryResultSet(query)
	resultSet.Results.Store(result.Id, &QueryResultCache{
		Result:         result,
		PluginInstance: pluginInstance,
		Query:          query,
	})
	sessionQueries.Store(query.Id, resultSet)
	manager.sessionQueryResultCache.Store(query.SessionId, sessionQueries)

	return manager, pluginInstance
}

func TestShouldClearGroupForGlobalQueryKeepsFilePlugin(t *testing.T) {
	globalQuery := Query{Type: QueryTypeInput, Search: "scottqian"}
	filePlugin := &Instance{Metadata: Metadata{Id: fileSearchPluginID}}
	otherPlugin := &Instance{Metadata: Metadata{Id: "other-plugin"}}

	assert.False(t, shouldClearGroupForGlobalQuery(globalQuery, filePlugin))
	assert.True(t, shouldClearGroupForGlobalQuery(globalQuery, otherPlugin))
	assert.False(t, shouldClearGroupForGlobalQuery(Query{Type: QueryTypeInput, TriggerKeyword: "f"}, filePlugin))
}

func TestShouldHidePreviewForGlobalQueryStillHidesFilePreview(t *testing.T) {
	globalQuery := Query{Type: QueryTypeInput, Search: "scottqian"}
	preview := WoxPreview{PreviewType: WoxPreviewTypeFile, PreviewData: `C:\tmp\readme.txt`}

	assert.True(t, shouldHidePreviewForGlobalQuery(globalQuery, preview))
	assert.False(t, shouldHidePreviewForGlobalQuery(Query{Type: QueryTypeInput, TriggerKeyword: "f"}, preview))
}

func TestBuildQueryResultsSnapshotKeepsUngroupedResultsAboveFileGroup(t *testing.T) {
	query := Query{Id: "query-1", SessionId: "session-1", Type: QueryTypeInput, Search: "scottqian"}
	manager := &Manager{
		sessionQueryResultCache: util.NewHashMap[string, *util.HashMap[string, *QueryResultSet]](),
	}
	resultSet := newQueryResultSet(query)
	resultSet.Results.Store("app", &QueryResultCache{
		Result: QueryResult{Id: "app", Title: "App", Score: 80},
	})
	resultSet.Results.Store("file", &QueryResultCache{
		Result: QueryResult{Id: "file", Title: "readme.txt", Score: 200, Group: "Files", GroupScore: 0},
	})
	sessionQueries := util.NewHashMap[string, *QueryResultSet]()
	sessionQueries.Store(query.Id, resultSet)
	manager.sessionQueryResultCache.Store(query.SessionId, sessionQueries)

	results := manager.BuildQueryResultsSnapshot(query.SessionId, query.Id)
	if len(results) != 3 {
		t.Fatalf("snapshot length = %d, want 3", len(results))
	}
	if results[0].Id != "app" || results[0].IsGroup {
		t.Fatalf("first result = %#v, want ungrouped app", results[0])
	}
	if !results[1].IsGroup || results[1].Title != "Files" {
		t.Fatalf("second result = %#v, want Files group header", results[1])
	}
	if results[2].Id != "file" || results[2].IsGroup {
		t.Fatalf("third result = %#v, want file result under group", results[2])
	}
}
