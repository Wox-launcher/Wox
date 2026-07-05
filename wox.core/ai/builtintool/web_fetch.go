package tool

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"wox/ai"
	"wox/common"
	"wox/setting"

	"github.com/tmc/langchaingo/jsonschema"
)

var (
	htmlBlockRegexp = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>|<noscript[^>]*>.*?</noscript>`)
	htmlTagRegexp   = regexp.MustCompile(`(?s)<[^>]+>`)
	spaceRegexp     = regexp.MustCompile(`\s+`)
)

func init() {
	ai.GetToolRegistry().Register(WebFetchTool())
}

// WebFetchTool fetches a web page through Exa's hosted MCP fetch tool or a local HTTP fallback.
func WebFetchTool() common.Tool {
	return common.Tool{
		Name:        ai.WebFetchToolName,
		Description: "Fetch readable content from an http or https URL. Uses Exa extraction when available and local HTTP fetch otherwise.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"url": {
					Type:        jsonschema.String,
					Description: "The http or https URL to fetch.",
				},
				"max_characters": {
					Type:        jsonschema.Integer,
					Description: "Optional maximum characters to return.",
				},
			},
			Required: []string{"url"},
		},
		Source:   common.ToolSourceBuiltin,
		Callback: webFetchCallback,
	}
}

func webFetchCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	rawURL, _ := args["url"].(string)
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return common.ToolResult{}, fmt.Errorf("url is required")
	}

	config := defaultAIWebSearchConfig()
	maxCharacters := argumentInt(args, "max_characters", config.FetchMaxCharacters)
	maxCharacters = clampInt(maxCharacters, 1000, setting.MaxAIWebSearchFetchMaxCharacters)
	content, err := fetchExa(ctx, config, rawURL, maxCharacters)
	if err != nil {
		return common.ToolResult{}, err
	}
	return common.ToolResult{Text: formatWebFetchResult(rawURL, maxCharacters, content)}, nil
}

// fetchExa calls Exa's hosted MCP fetch tool for provider-side extraction.
func fetchExa(ctx context.Context, config aiWebSearchConfig, rawURL string, maxCharacters int) (string, error) {
	text, err := callExaMCPTool(ctx, config, exaWebFetchToolName, map[string]any{
		"urls":          []string{rawURL},
		"maxCharacters": maxCharacters,
	})
	if err != nil {
		return "", err
	}
	return truncateString(text, maxCharacters), nil
}

// validateHTTPURL prevents local file or custom-scheme fetches from entering the web_fetch tool path.
func validateHTTPURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("only http and https URLs are supported")
	}
	if parsed.Host == "" {
		return fmt.Errorf("url host is required")
	}
	return nil
}

// formatWebFetchResult produces the model-facing text returned by web_fetch.
func formatWebFetchResult(rawURL string, maxCharacters int, content string) string {
	var builder strings.Builder
	builder.WriteString("# Web Fetch Result\n")
	builder.WriteString("Provider: exa")
	builder.WriteString("\nURL: ")
	builder.WriteString(rawURL)
	builder.WriteString(fmt.Sprintf("\nMax Characters: %d\n\n", maxCharacters))
	builder.WriteString(truncateString(content, maxCharacters))
	return strings.TrimSpace(builder.String())
}

// htmlToText removes noisy markup before returning locally fetched pages to the model.
func htmlToText(content string) string {
	content = htmlBlockRegexp.ReplaceAllString(content, " ")
	content = htmlTagRegexp.ReplaceAllString(content, " ")
	content = html.UnescapeString(content)
	content = spaceRegexp.ReplaceAllString(content, " ")
	return strings.TrimSpace(content)
}

// localHTTPFetch is the safe fallback when provider-side extraction is unavailable.
func localHTTPFetch(ctx context.Context, rawURL string, maxCharacters int) (string, error) {
	if err := validateHTTPURL(rawURL); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Wox/2.0 AI web_fetch")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain,application/json;q=0.9,*/*;q=0.8")

	client := webHTTPClient(ctx, 30*time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("fetch failed with HTTP status %d", resp.StatusCode)
	}

	readLimit := clampInt(maxCharacters*4, maxCharacters+4096, setting.MaxAIWebSearchFetchMaxCharacters*2)
	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(readLimit)))
	if err != nil {
		return "", err
	}

	content := string(data)
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "html") || strings.Contains(strings.ToLower(content[:minInt(len(content), 512)]), "<html") {
		content = htmlToText(content)
	}
	return truncateString(content, maxCharacters), nil
}
