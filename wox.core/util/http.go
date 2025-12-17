package util

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

var (
	httpClient  *http.Client
	clientMutex sync.Mutex
	defaultUA   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/132.0.0.0 Safari/537.36"

	// fallbackDNSServers are used when the primary DNS resolution fails (e.g., "no such host").
	fallbackDNSServers = []string{"1.1.1.1:53", "8.8.8.8:53"}
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
	resp, err := doRequestWithClient(req, getClient(), true)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("http %s %s failed, status code: %d", req.Method, req.URL, resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func doRequestWithClient(req *http.Request, client *http.Client, allowFallback bool) (*http.Response, error) {
	resp, err := client.Do(req)
	if err != nil && allowFallback && shouldRetryWithFallback(err) {
		fallbackClient := getFallbackHTTPClient(req.Context())
		if fallbackClient != nil {
			GetLogger().Warn(req.Context(), fmt.Sprintf("dns resolution failed, retrying with fallback resolver: %v", err))
			return doRequestWithClient(req.Clone(req.Context()), fallbackClient, false)
		}
	}
	return resp, err
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
	return HttpDownloadWithProgress(ctx, url, dest, nil)
}

// HttpDownloadWithProgress downloads a file from url to dest with optional progress callback
// progressCallback receives (downloaded bytes, total bytes). Total bytes may be -1 if Content-Length is not available.
func HttpDownloadWithProgress(ctx context.Context, url string, dest string, progressCallback func(downloaded int64, total int64)) error {
	req, err := newRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	return httpDownloadWithClient(ctx, req, dest, progressCallback, getClient(), true)
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

func shouldRetryWithFallback(err error) bool {
	// Check if the error is a DNS error, see #4303
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
}

func getFallbackHTTPClient(ctx context.Context) *http.Client {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	baseClient := getClient()

	var baseTransport *http.Transport
	switch t := baseClient.Transport.(type) {
	case nil:
		baseTransport = http.DefaultTransport.(*http.Transport)
	case *http.Transport:
		baseTransport = t
	default:
		GetLogger().Warn(ctx, fmt.Sprintf("unsupported transport type, skip DNS fallback: %T", baseClient.Transport))
		return nil
	}

	fallbackTransport := baseTransport.Clone()
	fallbackTransport.DialContext = (&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		Resolver:  newFallbackResolver(),
	}).DialContext

	return &http.Client{
		Transport: fallbackTransport,
		Timeout:   baseClient.Timeout,
	}
}

func newFallbackResolver() *net.Resolver {
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var lastErr error
			d := net.Dialer{Timeout: 5 * time.Second}
			for _, dnsAddr := range fallbackDNSServers {
				conn, err := d.DialContext(ctx, network, dnsAddr)
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			if lastErr != nil {
				return nil, lastErr
			}
			return nil, fmt.Errorf("all fallback DNS servers failed")
		},
	}
}

func httpDownloadWithClient(ctx context.Context, req *http.Request, dest string, progressCallback func(downloaded int64, total int64), client *http.Client, allowFallback bool) error {
	resp, err := client.Do(req)
	if err != nil {
		if allowFallback && shouldRetryWithFallback(err) {
			fallbackClient := getFallbackHTTPClient(ctx)
			if fallbackClient != nil {
				GetLogger().Warn(ctx, fmt.Sprintf("dns resolution failed, retrying download with fallback resolver: %v", err))
				return httpDownloadWithClient(ctx, req.Clone(ctx), dest, progressCallback, fallbackClient, false)
			}
		}
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("http download %s failed, status code: %d", req.URL, resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get total size from Content-Length header (may be -1 if not available)
	totalSize := resp.ContentLength

	// If no progress callback provided, use simple copy
	if progressCallback == nil {
		_, err = io.Copy(out, resp.Body)
		return err
	}

	// Use progress tracking copy
	var downloaded int64
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		nr, er := resp.Body.Read(buf)
		if nr > 0 {
			nw, ew := out.Write(buf[0:nr])
			if nw > 0 {
				downloaded += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}

			// Call progress callback
			progressCallback(downloaded, totalSize)
		}
		if er != nil {
			if er != io.EOF {
				err = er
			}
			break
		}
	}

	return err
}
