package util

import (
	"context"
	"net/http"
	"testing"
	"wox/network"
)

func TestUpdateHTTPProxyReplacesAndClearsExplicitProxy(t *testing.T) {
	if err := GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("https_proxy", "")

	clientMutex.Lock()
	previous := httpClient
	httpClient = nil
	clientMutex.Unlock()
	t.Cleanup(func() {
		clientMutex.Lock()
		httpClient = previous
		clientMutex.Unlock()
	})

	ctx := context.Background()
	const rawURL = "https://sync.woxlauncher.com/v1/sync/devices/list"
	ApplyHTTPProxy(ctx, true, "http://127.0.0.1:7890")
	if got := proxyFor(t, rawURL); got != "http://127.0.0.1:7890" {
		t.Fatalf("proxy = %q, want http://127.0.0.1:7890", got)
	}

	ApplyHTTPProxy(ctx, true, "http://127.0.0.1:8080")
	if got := proxyFor(t, rawURL); got != "http://127.0.0.1:8080" {
		t.Fatalf("proxy = %q, want http://127.0.0.1:8080", got)
	}

	ApplyHTTPProxy(ctx, false, "http://127.0.0.1:8080")
	if got := proxyFor(t, rawURL); got != "" {
		t.Fatalf("proxy after disable = %q, want direct", got)
	}
}

func proxyFor(t *testing.T, rawURL string) string {
	t.Helper()
	client := GetHTTPClient(context.Background())
	transport, ok := network.BaseTransport(client.Transport).(*http.Transport)
	if !ok || transport == nil || transport.Proxy == nil {
		return ""
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	proxyURL, err := transport.Proxy(req)
	if err != nil {
		t.Fatal(err)
	}
	if proxyURL == nil {
		return ""
	}
	return proxyURL.String()
}
