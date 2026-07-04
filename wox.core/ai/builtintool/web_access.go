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

	"wox/setting"
	"wox/util"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	exaWebSearchToolName = "web_search_exa"
	exaWebFetchToolName  = "web_fetch_exa"
)

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
