package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"wox/ai"
	"wox/common"
	"wox/setting"

	"github.com/tmc/langchaingo/jsonschema"
)

func init() {
	ai.GetToolRegistry().Register(WebSearchTool())
}

type webSearchResult struct {
	Title    string
	URL      string
	Snippet  string
	Content  string
	Source   string
	Provider string
}

// WebSearchTool searches the web through the configured AI web access provider.
func WebSearchTool() common.Tool {
	return common.Tool{
		Name:        ai.WebSearchToolName,
		Description: "Search the web for current or external information. Returns normalized search results with title, url, snippet, content, source, and provider.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"query": {
					Type:        jsonschema.String,
					Description: "Search query.",
				},
				"num_results": {
					Type:        jsonschema.Integer,
					Description: "Optional number of results to return.",
				},
			},
			Required: []string{"query"},
		},
		Source:   common.ToolSourceBuiltin,
		Callback: webSearchCallback,
	}
}

func webSearchCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	query, _ := args["query"].(string)
	query = strings.TrimSpace(query)
	if query == "" {
		return common.ToolResult{}, fmt.Errorf("query is required")
	}

	config, err := currentAIWebSearchConfig(ctx)
	if err != nil {
		return common.ToolResult{}, err
	}

	count := argumentInt(args, "num_results", config.SearchResultCount)
	count = clampInt(count, 1, setting.MaxAIWebSearchResultCount)
	results, err := searchWeb(ctx, config, query, count)
	if err != nil {
		return common.ToolResult{}, err
	}
	return common.ToolResult{Text: formatWebSearchResults(query, config.Provider, results)}, nil
}

// searchWeb dispatches a normalized search request to the configured provider adapter.
func searchWeb(ctx context.Context, config setting.AIWebSearchConfig, query string, count int) ([]webSearchResult, error) {
	switch setting.AIWebSearchProvider(config.Provider) {
	case setting.AIWebSearchProviderExa:
		return searchExa(ctx, config, query, count)
	case setting.AIWebSearchProviderTavily:
		return searchTavily(ctx, config, query, count)
	case setting.AIWebSearchProviderBrave:
		return searchBrave(ctx, config, query, count)
	case setting.AIWebSearchProviderSearXNG:
		return searchSearXNG(ctx, config, query, count)
	default:
		return nil, fmt.Errorf("unsupported AI web search provider: %s", config.Provider)
	}
}

// searchExa calls Exa's hosted MCP search tool and normalizes its response when it is structured.
func searchExa(ctx context.Context, config setting.AIWebSearchConfig, query string, count int) ([]webSearchResult, error) {
	text, err := callExaMCPTool(ctx, config, exaWebSearchToolName, map[string]any{
		"query":      query,
		"numResults": count,
	})
	if err != nil {
		return nil, err
	}
	results := parseGenericSearchResults("exa", []byte(text), count)
	if len(results) > 0 {
		return results, nil
	}
	return []webSearchResult{{
		Title:    "Exa search results",
		Content:  truncateString(text, config.FetchMaxCharacters),
		Source:   "exa-mcp",
		Provider: "exa",
	}}, nil
}

// searchTavily maps Tavily /search results into Wox's provider-neutral shape.
func searchTavily(ctx context.Context, config setting.AIWebSearchConfig, query string, count int) ([]webSearchResult, error) {
	type tavilyResult struct {
		Title      string `json:"title"`
		URL        string `json:"url"`
		Content    string `json:"content"`
		RawContent string `json:"raw_content"`
	}
	type tavilyResponse struct {
		Results []tavilyResult `json:"results"`
		Answer  string         `json:"answer"`
	}

	var response tavilyResponse
	err := postJSON(ctx, joinEndpointPath(config.Endpoint, "/search"), map[string]string{
		"Authorization": "Bearer " + config.ApiKey,
	}, map[string]any{
		"query":               query,
		"max_results":         count,
		"include_raw_content": true,
	}, &response)
	if err != nil {
		return nil, err
	}

	results := make([]webSearchResult, 0, len(response.Results))
	for _, item := range response.Results {
		results = append(results, webSearchResult{
			Title:    item.Title,
			URL:      item.URL,
			Snippet:  item.Content,
			Content:  item.RawContent,
			Source:   "tavily",
			Provider: "tavily",
		})
	}
	if len(results) == 0 && strings.TrimSpace(response.Answer) != "" {
		results = append(results, webSearchResult{Title: "Tavily answer", Content: response.Answer, Source: "tavily", Provider: "tavily"})
	}
	return results, nil
}

// searchBrave calls Brave's LLM context endpoint and keeps a text fallback for response shape drift.
func searchBrave(ctx context.Context, config setting.AIWebSearchConfig, query string, count int) ([]webSearchResult, error) {
	endpointURL, err := url.Parse(joinEndpointPath(config.Endpoint, "/res/v1/llm/context"))
	if err != nil {
		return nil, err
	}
	q := endpointURL.Query()
	q.Set("q", query)
	q.Set("count", fmt.Sprintf("%d", count))
	endpointURL.RawQuery = q.Encode()

	data, err := getBytes(ctx, endpointURL.String(), map[string]string{
		"Accept":               "application/json",
		"X-Subscription-Token": config.ApiKey,
	})
	if err != nil {
		return nil, err
	}
	results := parseGenericSearchResults("brave", data, count)
	if len(results) > 0 {
		return results, nil
	}
	return []webSearchResult{{
		Title:    "Brave LLM context",
		Content:  truncateString(string(data), config.FetchMaxCharacters),
		Source:   "brave",
		Provider: "brave",
	}}, nil
}

// searchSearXNG calls the configured instance's JSON search API.
func searchSearXNG(ctx context.Context, config setting.AIWebSearchConfig, query string, count int) ([]webSearchResult, error) {
	endpointURL, err := url.Parse(joinEndpointPath(config.Endpoint, "/search"))
	if err != nil {
		return nil, err
	}
	q := endpointURL.Query()
	q.Set("q", query)
	q.Set("format", "json")
	endpointURL.RawQuery = q.Encode()

	type searxngResult struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
		Engine  string `json:"engine"`
	}
	type searxngResponse struct {
		Results []searxngResult `json:"results"`
	}

	data, err := getBytes(ctx, endpointURL.String(), map[string]string{"Accept": "application/json"})
	if err != nil {
		return nil, err
	}

	var response searxngResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}

	results := make([]webSearchResult, 0, len(response.Results))
	for _, item := range response.Results {
		if len(results) >= count {
			break
		}
		results = append(results, webSearchResult{
			Title:    item.Title,
			URL:      item.URL,
			Snippet:  item.Content,
			Source:   item.Engine,
			Provider: "searxng",
		})
	}
	return results, nil
}

// parseGenericSearchResults extracts common result arrays from provider-specific JSON payloads.
func parseGenericSearchResults(provider string, data []byte, limit int) []webSearchResult {
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}

	items := findResultItems(payload)
	results := make([]webSearchResult, 0, len(items))
	for _, item := range items {
		if len(results) >= limit {
			break
		}
		title := firstString(item, "title", "name")
		resultURL := firstString(item, "url", "link")
		snippet := firstString(item, "snippet", "description", "content", "text")
		content := firstString(item, "raw_content", "rawContent", "markdown", "summary")
		if title == "" && resultURL == "" && snippet == "" && content == "" {
			continue
		}
		results = append(results, webSearchResult{
			Title:    title,
			URL:      resultURL,
			Snippet:  snippet,
			Content:  content,
			Source:   firstString(item, "source", "engine"),
			Provider: provider,
		})
	}
	return results
}

// findResultItems locates the most common search-result array keys used by web providers.
func findResultItems(payload any) []map[string]any {
	switch value := payload.(type) {
	case []any:
		return mapsFromArray(value)
	case map[string]any:
		for _, key := range []string{"results", "items", "sources", "data", "organic_results"} {
			if raw, ok := value[key]; ok {
				if array, ok := raw.([]any); ok {
					return mapsFromArray(array)
				}
			}
		}
		if web, ok := value["web"].(map[string]any); ok {
			if raw, ok := web["results"].([]any); ok {
				return mapsFromArray(raw)
			}
		}
	}
	return nil
}

// mapsFromArray keeps only object items from a loosely typed JSON array.
func mapsFromArray(values []any) []map[string]any {
	results := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if item, ok := value.(map[string]any); ok {
			results = append(results, item)
		}
	}
	return results
}

// firstString returns the first non-empty string-like field from a parsed result item.
func firstString(item map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := item[key]; ok {
			switch typed := value.(type) {
			case string:
				if strings.TrimSpace(typed) != "" {
					return strings.TrimSpace(typed)
				}
			case json.Number:
				return typed.String()
			}
		}
	}
	return ""
}

// formatWebSearchResults produces the model-facing text returned by web_search.
func formatWebSearchResults(query string, provider string, results []webSearchResult) string {
	if len(results) == 0 {
		return fmt.Sprintf("No web search results from %s for query: %s", provider, query)
	}

	var builder strings.Builder
	builder.WriteString("# Web Search Results\n")
	builder.WriteString("Provider: ")
	builder.WriteString(provider)
	builder.WriteString("\nQuery: ")
	builder.WriteString(query)
	builder.WriteString("\n\n")
	for i, result := range results {
		builder.WriteString(fmt.Sprintf("## %d. %s\n", i+1, emptyFallback(result.Title, "Untitled result")))
		if result.URL != "" {
			builder.WriteString("URL: ")
			builder.WriteString(result.URL)
			builder.WriteString("\n")
		}
		if result.Snippet != "" {
			builder.WriteString("Snippet: ")
			builder.WriteString(result.Snippet)
			builder.WriteString("\n")
		}
		if result.Content != "" {
			builder.WriteString("Content: ")
			builder.WriteString(truncateString(result.Content, 2400))
			builder.WriteString("\n")
		}
		if result.Source != "" {
			builder.WriteString("Source: ")
			builder.WriteString(result.Source)
			builder.WriteString("\n")
		}
		builder.WriteString("Provider: ")
		builder.WriteString(emptyFallback(result.Provider, provider))
		builder.WriteString("\n\n")
	}
	return strings.TrimSpace(builder.String())
}
