package plugin

import (
	"context"
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
