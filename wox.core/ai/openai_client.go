package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"wox/network"
	"wox/util"
)

const (
	openAIMaxRetries       = 2
	openAIMaxErrorBodySize = 64 * 1024
	openAIMaxRetryDelay    = 5 * time.Second
)

// ProviderAPIError represents a non-2xx HTTP response returned by an AI provider API.
type ProviderAPIError struct {
	Method     string
	URL        string
	StatusCode int
	Status     string
	Body       string
}

func (e *ProviderAPIError) Error() string {
	status := strings.TrimSpace(e.Status)
	if status == "" && e.StatusCode > 0 {
		if text := http.StatusText(e.StatusCode); text != "" {
			status = fmt.Sprintf("%d %s", e.StatusCode, text)
		} else {
			status = strconv.Itoa(e.StatusCode)
		}
	}

	base := strings.TrimSpace(fmt.Sprintf("%s %s: %s", e.Method, e.URL, status))
	if trimmed := strings.TrimSpace(e.Body); trimmed != "" {
		const maxBodyLen = 4096
		if len(trimmed) > maxBodyLen {
			trimmed = trimmed[:maxBodyLen] + "..."
		}
		return fmt.Sprintf("%s\n%s", base, trimmed)
	}
	return base
}

type openAITool struct {
	Type     string             `json:"type"`
	Function openAIToolFunction `json:"function"`
}

type openAIToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type openAIChatMessage struct {
	Role             string                  `json:"role"`
	Content          any                     `json:"content,omitempty"`
	ToolCalls        []openAIMessageToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string                  `json:"tool_call_id,omitempty"`
	ReasoningContent *string                 `json:"reasoning_content,omitempty"`
}

type openAIContentPart struct {
	Type     string              `json:"type"`
	Text     string              `json:"text,omitempty"`
	ImageURL *openAIContentImage `json:"image_url,omitempty"`
}

type openAIContentImage struct {
	URL string `json:"url"`
}

type openAIMessageToolCall struct {
	ID       string                    `json:"id"`
	Type     string                    `json:"type"`
	Function openAIMessageToolCallFunc `json:"function"`
}

type openAIMessageToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIModelsListResponse struct {
	Data []openAIModelObject `json:"data"`
}

type openAIModelObject struct {
	ID string `json:"id"`
}

type openAIChatCompletionChunk struct {
	Choices []openAIChunkChoice `json:"choices"`
	Error   *openAIStreamError  `json:"error,omitempty"`
}

type openAIStreamError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code"`
}

type openAIChunkChoice struct {
	Index int              `json:"index"`
	Delta openAIChunkDelta `json:"delta"`
}

type openAIChunkDelta struct {
	Content          string                `json:"content"`
	Refusal          string                `json:"refusal"`
	Reasoning        json.RawMessage       `json:"reasoning"`
	ReasoningContent json.RawMessage       `json:"reasoning_content"`
	ToolCalls        []openAIChunkToolCall `json:"tool_calls"`
}

type openAIChunkToolCall struct {
	Index    int                     `json:"index"`
	ID       string                  `json:"id"`
	Type     string                  `json:"type"`
	Function openAIChunkToolCallFunc `json:"function"`
}

type openAIChunkToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIAccumulatedToolCall struct {
	ID        string
	Name      string
	Arguments string
}

func stringPtr(s string) *string {
	return &s
}

func (d openAIChunkDelta) reasoningRawField(name string) (json.RawMessage, bool) {
	var raw json.RawMessage
	switch name {
	case "reasoning":
		raw = d.Reasoning
	case "reasoning_content":
		raw = d.ReasoningContent
	default:
		return nil, false
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, false
	}
	return trimmed, true
}

func (o *OpenAIBaseProvider) buildEndpointURL(endpoint string, queryParams map[string]string) (string, error) {
	rawHost := strings.TrimSpace(o.connectContext.Host)
	if rawHost == "" {
		return "", errors.New("AI provider host is empty")
	}

	parsed, err := url.Parse(rawHost)
	if err != nil {
		return "", fmt.Errorf("invalid AI provider host %q: %w", rawHost, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid AI provider host %q", rawHost)
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + strings.TrimLeft(endpoint, "/")
	if len(queryParams) > 0 {
		q := parsed.Query()
		for k, v := range queryParams {
			q.Set(k, v)
		}
		parsed.RawQuery = q.Encode()
	}

	return parsed.String(), nil
}

func (o *OpenAIBaseProvider) doRequest(ctx context.Context, method string, endpoint string, queryParams map[string]string, bodyBytes []byte, accept string) (*http.Response, error) {
	targetURL, err := o.buildEndpointURL(endpoint, queryParams)
	if err != nil {
		return nil, err
	}

	client := util.GetHTTPClient(ctx)
	maxRetries := openAIMaxRetries
	if network.IsOffline() {
		maxRetries = 0
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		var bodyReader io.Reader
		if len(bodyBytes) > 0 {
			bodyReader = bytes.NewReader(bodyBytes)
		}

		req, reqErr := http.NewRequestWithContext(ctx, method, targetURL, bodyReader)
		if reqErr != nil {
			return nil, reqErr
		}

		// Keep headers minimal; some OpenAI-compatible relays reject SDK fingerprint headers.
		req.Header.Set("User-Agent", "Wox")
		if apiKey := strings.TrimSpace(o.connectContext.ApiKey); apiKey != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey)
		}
		if len(bodyBytes) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		for k, v := range o.options.Headers {
			req.Header.Set(k, v)
		}

		resp, doErr := client.Do(req)
		if doErr != nil {
			return nil, doErr
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}

		if attempt < maxRetries && isRetryableHTTPStatus(resp.StatusCode) {
			retryAfter := resp.Header.Get("Retry-After")
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, openAIMaxErrorBodySize))
			_ = resp.Body.Close()

			if waitErr := waitRetryDelay(ctx, attempt, retryAfter); waitErr != nil {
				return nil, waitErr
			}
			continue
		}

		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, openAIMaxErrorBodySize))
		_ = resp.Body.Close()
		return nil, &ProviderAPIError{
			Method:     method,
			URL:        targetURL,
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       string(errBody),
		}
	}

	return nil, errors.New("request attempts exhausted")
}

func (o *OpenAIBaseProvider) doJSONRequest(ctx context.Context, method string, endpoint string, queryParams map[string]string, reqBody any, respOut any) error {
	var bodyBytes []byte
	if reqBody != nil {
		encoded, err := json.Marshal(reqBody)
		if err != nil {
			return err
		}
		bodyBytes = encoded
	}

	resp, err := o.doRequest(ctx, method, endpoint, queryParams, bodyBytes, "application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if respOut == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, openAIMaxErrorBodySize))
		return nil
	}

	return json.NewDecoder(resp.Body).Decode(respOut)
}

func isRetryableHTTPStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func waitRetryDelay(ctx context.Context, attempt int, retryAfter string) error {
	delay := time.Duration(500*(1<<attempt)) * time.Millisecond
	if retryAfter != "" {
		if secs, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && secs > 0 {
			delay = time.Duration(secs) * time.Second
		}
	}
	if delay > openAIMaxRetryDelay {
		delay = openAIMaxRetryDelay
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type sseReader struct {
	body   io.ReadCloser
	reader *bufio.Reader
	closed bool
}

func newSSEReader(body io.ReadCloser) *sseReader {
	return &sseReader{
		body:   body,
		reader: bufio.NewReader(body),
	}
}

func (r *sseReader) close() {
	if !r.closed {
		r.closed = true
		_ = r.body.Close()
	}
}

// nextEvent reads the next SSE data payload using bufio.Reader so payloads > 64KB succeed.
func (r *sseReader) nextEvent() ([]byte, error) {
	if r.closed {
		return nil, io.EOF
	}

	var dataBuf []byte
	for {
		line, err := r.reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimRight(line, "\r\n")
			if len(trimmed) == 0 {
				if len(dataBuf) > 0 {
					if bytes.Equal(bytes.TrimSpace(dataBuf), []byte("[DONE]")) {
						r.close()
						return nil, io.EOF
					}
					if err != nil {
						r.close()
					}
					return dataBuf, nil
				}
			} else if trimmed[0] != ':' {
				if bytes.HasPrefix(trimmed, []byte("data:")) {
					val := trimmed[len("data:"):]
					if len(val) > 0 && val[0] == ' ' {
						val = val[1:]
					}
					if len(dataBuf) > 0 {
						dataBuf = append(dataBuf, '\n')
					}
					dataBuf = append(dataBuf, val...)
				}
			}
		}

		if err != nil {
			r.close()
			if errors.Is(err, io.EOF) {
				if len(dataBuf) > 0 && !bytes.Equal(bytes.TrimSpace(dataBuf), []byte("[DONE]")) {
					return dataBuf, nil
				}
				return nil, io.EOF
			}
			return nil, err
		}
	}
}
