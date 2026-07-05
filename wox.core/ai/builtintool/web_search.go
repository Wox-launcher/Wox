package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"wox/ai"
	"wox/common"
	"wox/setting"
	"wox/util"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tmc/langchaingo/jsonschema"
)

const exaWebSearchToolName = "web_search_exa"
const exaWebFetchToolName = "web_fetch_exa"

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

// searchTavily maps Tavily /search results into a provider-neutral shape.
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

// --- Shared web access helpers (used by web_search and web_fetch) ---

// currentAIWebSearchConfig reads and validates the user-facing web access setting before every tool call.
func currentAIWebSearchConfig(ctx context.Context) (setting.AIWebSearchConfig, error) {
	config := setting.NormalizeAIWebSearchConfig(setting.GetSettingManager().GetWoxSetting(ctx).AIWebSearch.Get())
	if !config.Enabled {
		return config, fmt.Errorf("AI web search is disabled")
	}
	switch setting.AIWebSearchProvider(config.Provider) {
	case setting.AIWebSearchProviderTavily, setting.AIWebSearchProviderBrave:
		if strings.TrimSpace(config.ApiKey) == "" {
			return config, fmt.Errorf("%s AI web search requires an API key", config.Provider)
		}
	case setting.AIWebSearchProviderSearXNG:
		if strings.TrimSpace(config.Endpoint) == "" {
			return config, fmt.Errorf("searxng AI web search requires a configured endpoint")
		}
	}
	return config, nil
}

// callExaMCPTool opens a short-lived hosted MCP session so Exa stays independent from user MCP settings.
func callExaMCPTool(ctx context.Context, config setting.AIWebSearchConfig, toolName string, args map[string]any) (string, error) {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "Wox",
		Version: "2.0.0",
	}, nil)

	httpClient := webHTTPClient(ctx, 45*time.Second)
	if config.ApiKey != "" {
		baseTransport := httpClient.Transport
		if baseTransport == nil {
			baseTransport = http.DefaultTransport
		}
		httpClient.Transport = headerRoundTripper{
			base: baseTransport,
			headers: map[string]string{
				"x-api-key": config.ApiKey,
			},
		}
	}

	endpoint := strings.TrimSpace(config.Endpoint)
	if endpoint == "" {
		endpoint = setting.DefaultAIWebSearchExaEndpoint
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	session, err := client.Connect(timeoutCtx, &mcp.StreamableClientTransport{Endpoint: endpoint, HTTPClient: httpClient, MaxRetries: -1}, nil)
	if err != nil {
		return "", err
	}
	defer session.Close()

	result, err := session.CallTool(timeoutCtx, &mcp.CallToolParams{Name: toolName, Arguments: args})
	if err != nil {
		return "", err
	}
	if result.IsError {
		return "", fmt.Errorf("exa MCP tool %s returned an error: %s", toolName, mcpContentToText(result))
	}
	return mcpContentToText(result), nil
}

// postJSON sends provider JSON requests without logging sensitive headers.
func postJSON(ctx context.Context, endpoint string, headers map[string]string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := webHTTPClient(ctx, 30*time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("request failed with HTTP status %d: %s", resp.StatusCode, truncateString(string(data), 800))
	}
	if target == nil {
		return nil
	}
	return json.Unmarshal(data, target)
}

// getBytes sends provider GET requests and returns bounded response data.
func getBytes(ctx context.Context, endpoint string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := webHTTPClient(ctx, 30*time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("request failed with HTTP status %d: %s", resp.StatusCode, truncateString(string(data), 800))
	}
	return data, nil
}

// joinEndpointPath preserves custom instance base paths while appending provider API routes.
func joinEndpointPath(endpoint string, path string) string {
	endpoint = strings.TrimSpace(endpoint)
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimRight(endpoint, "/") + path
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

// mcpContentToText flattens Exa MCP content into a plain text tool result.
func mcpContentToText(result *mcp.CallToolResult) string {
	var parts []string
	for _, content := range result.Content {
		if text, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, text.Text)
			continue
		}
		if data, err := json.Marshal(content); err == nil {
			parts = append(parts, string(data))
		}
	}
	if len(parts) == 0 && result.StructuredContent != nil {
		if data, err := json.Marshal(result.StructuredContent); err == nil {
			parts = append(parts, string(data))
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n"))
}

// argumentInt reads optional integer tool arguments from the provider's loose JSON map.
func argumentInt(args map[string]any, key string, fallback int) int {
	switch value := args[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		if parsed, err := value.Int64(); err == nil {
			return int(parsed)
		}
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return fallback
}

// truncateString caps provider output before it enters the model context.
func truncateString(value string, maxCharacters int) string {
	value = strings.TrimSpace(value)
	if maxCharacters <= 0 || len(value) <= maxCharacters {
		return value
	}
	return strings.TrimSpace(value[:maxCharacters]) + "\n...[truncated]"
}

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func emptyFallback(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// webHTTPClient reuses Wox proxy settings while giving web tools a bounded timeout.
func webHTTPClient(ctx context.Context, timeout time.Duration) *http.Client {
	base := util.GetHTTPClient(ctx)
	clone := *base
	clone.Timeout = timeout
	return &clone
}

type headerRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	for key, value := range t.headers {
		clone.Header.Set(key, value)
	}
	if t.base == nil {
		return http.DefaultTransport.RoundTrip(clone)
	}
	return t.base.RoundTrip(clone)
}