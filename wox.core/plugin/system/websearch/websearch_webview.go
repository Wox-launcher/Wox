package websearch

import (
	"context"
	"encoding/json"
	"strings"
	"wox/common"
	"wox/plugin"

	"github.com/google/uuid"
)

const webSearchPreviewContextKey = "websearch:preview"

// webSearchAutomaticPreview supplies the page independently of the manual open action.
func webSearchAutomaticPreview(search webSearch, url string) plugin.WoxPreview {
	payload, _ := json.Marshal(plugin.WoxPreviewWebviewData{
		Url: url, InjectCss: strings.TrimSpace(search.InjectCSS),
		UserAgent: resolveWebviewUserAgent(search.UserAgent), CacheDisabled: !search.KeepInBackground,
	})
	return plugin.WoxPreview{
		PreviewType: plugin.WoxPreviewTypeWebView, PreviewData: string(payload),
	}
}

// webSearchResponse sends content and its preview-only layout in the same snapshot.
func webSearchResponse(results []plugin.QueryResult, search webSearch) plugin.QueryResponse {
	response := plugin.NewQueryResponse(results)
	if len(results) == 1 && !results[0].Preview.IsEmpty() {
		ratio := 0.0
		response.Layout.ResultPreviewWidthRatio = &ratio
		response.Layout.WindowWidth = max(0, search.WebViewWidth)
		response.Layout.WindowHeight = max(0, search.WebViewHeight)
	}
	return response
}

// webSearchPreviewRequest carries resolved inputs into a window for actions executed without a launcher.
func webSearchPreviewRequest(search webSearch, values map[string]string) common.OpenWoxInstanceRequest {
	data := webSearchMRUContext(search, values)
	data[webSearchPreviewContextKey] = "true"
	return common.OpenWoxInstanceRequest{
		Role: common.WoxInstanceRoleSecondary, InstanceName: "websearch:" + search.Keyword,
		Query:   common.PlainQuery{QueryId: uuid.NewString(), QueryType: plugin.QueryTypeInput, QueryText: search.Keyword + " ", ContextData: data},
		ShowApp: common.ShowContext{HideQueryBox: true, HideToolbar: true, ShowPreviewTitleBar: true, WindowWidth: search.WebViewWidth, ShowSource: common.ShowSourceQueryHotkey},
	}
}

// queryWebViewPreview restores captured inputs instead of reading selection or clipboard after focus changes.
func (r *WebSearchPlugin) queryWebViewPreview(ctx context.Context, search webSearch, data common.ContextData) plugin.QueryResponse {
	var values map[string]string
	if err := json.Unmarshal([]byte(data[webSearchMRUValuesKey]), &values); err != nil {
		return plugin.QueryResponse{}
	}
	if firstWebViewSearchURL(search, values) == "" {
		return plugin.QueryResponse{}
	}
	search.OpenInWebView = true
	result := r.searchResult(ctx, search, values, values, nil)
	return webSearchResponse([]plugin.QueryResult{result}, search)
}
