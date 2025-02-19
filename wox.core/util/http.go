package util

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
)

var (
	httpClient  *http.Client
	clientMutex sync.Mutex
	defaultUA   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36"
)

// newRequest creates a new http request with common headers
func newRequest(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUA)
	return req, nil
}

// doRequest executes the request and handles common response processing
func doRequest(req *http.Request) ([]byte, error) {
	resp, err := getClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http %s %s failed, status code: %d", req.Method, req.URL, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func HttpGet(ctx context.Context, url string) ([]byte, error) {
	req, err := newRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return doRequest(req)
}

func HttpGetWithHeaders(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	req, err := newRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return doRequest(req)
}

func HttpPost(ctx context.Context, url string, body any) ([]byte, error) {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := newRequest(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	return doRequest(req)
}

func HttpDownload(ctx context.Context, url string, dest string) error {
	req, err := newRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := getClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http download %s failed, status code: %d", url, resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func UpdateHTTPProxy(ctx context.Context, proxyUrl string) {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	GetLogger().Info(ctx, fmt.Sprintf("updating HTTP proxy, url: %s", proxyUrl))

	transport := &http.Transport{}
	if proxyUrl != "" {
		proxyURL, err := url.Parse(proxyUrl)
		if err != nil {
			GetLogger().Error(ctx, fmt.Sprintf("failed to parse proxy url: %s", err.Error()))
			return
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	httpClient = &http.Client{
		Transport: transport,
	}
}

func getClient() *http.Client {
	if httpClient == nil {
		httpClient = &http.Client{}
	}
	return httpClient
}

// GetHTTPClient returns a http client with proxy settings from context
func GetHTTPClient(ctx context.Context) *http.Client {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	return getClient()
}
