package network

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOfflineRejectsBeforeTransport(t *testing.T) {
	p := &Policy{}
	p.SetOffline(true)
	var called atomic.Int32
	client := &http.Client{Transport: p.Wrap(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called.Add(1)
		return nil, errors.New("unexpected transport")
	}))}
	for _, target := range []string{"https://example.com", "http://192.168.1.5", "http://10.0.0.1", "http://[fe80::1]", "http://localhost.example.com", "http://2130706433"} {
		_, err := client.Get(target)
		if !errors.Is(err, ErrOffline) {
			t.Fatalf("%s: %v", target, err)
		}
	}
	if called.Load() != 0 {
		t.Fatal("offline request reached transport")
	}
}

func TestLoopbackAndRedirectPolicy(t *testing.T) {
	p := &Policy{}
	p.SetOffline(true)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "https://example.com", http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "local")
	}))
	defer server.Close()
	proxyCalls := atomic.Int32{}
	base := http.DefaultTransport.(*http.Transport).Clone()
	base.Proxy = func(*http.Request) (*url.URL, error) { proxyCalls.Add(1); return url.Parse("http://example.com:9999") }
	client := &http.Client{Transport: p.Wrap(base)}
	for _, target := range []string{server.URL, strings.Replace(server.URL, "127.0.0.1", "localhost", 1)} {
		response, err := client.Get(target)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
	}
	if proxyCalls.Load() != 0 {
		t.Fatal("loopback used proxy")
	}
	_, err := client.Get(server.URL + "/redirect")
	if !errors.Is(err, ErrOffline) {
		t.Fatalf("redirect: %v", err)
	}
	for _, target := range []string{"http://127.0.0.2", "http://[::1]", "http://[::ffff:127.0.0.1]"} {
		u, _ := url.Parse(target)
		if !LocalURL(u) {
			t.Fatalf("loopback rejected: %s", target)
		}
	}
}

func TestToggleCancelsRequestAndAllowsFreshWork(t *testing.T) {
	p := &Policy{}
	started := make(chan struct{})
	done := make(chan error, 1)
	client := &http.Client{Transport: p.Wrap(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-r.Context().Done()
		return nil, r.Context().Err()
	}))}
	go func() { _, err := client.Get("https://example.com"); done <- err }()
	<-started
	p.SetOffline(true)
	select {
	case err := <-done:
		if !errors.Is(err, ErrOffline) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("request not cancelled")
	}
	p.SetOffline(false)
	ctx, release, err := p.Begin(context.Background())
	if err != nil || ctx.Err() != nil {
		t.Fatal("fresh operation failed", err)
	}
	release()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.active) != 0 {
		t.Fatal("operation registrations leaked")
	}
}

// A stream remains registered after headers until its body closes or reaches EOF.
func TestToggleCancelsStreamingBody(t *testing.T) {
	p := &Policy{}
	reader, writer := io.Pipe()
	client := &http.Client{Transport: p.Wrap(roundTripFunc(func(r *http.Request) (*http.Response, error) {
		go func() { <-r.Context().Done(); writer.CloseWithError(r.Context().Err()) }()
		return &http.Response{StatusCode: 200, Body: reader, Header: make(http.Header)}, nil
	}))}
	response, err := client.Get("https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	p.SetOffline(true)
	_, err = io.ReadAll(response.Body)
	if !errors.Is(err, ErrOffline) {
		t.Fatalf("stream: %v", err)
	}
}

func TestBeginAndToggleConcurrent(t *testing.T) {
	p := &Policy{}
	for range 100 {
		done := make(chan struct{})
		go func() {
			ctx, release, err := p.Begin(context.Background())
			if err == nil {
				release()
				<-ctx.Done()
			}
			close(done)
		}()
		p.SetOffline(true)
		<-done
		p.SetOffline(false)
	}
}

// TestRapidToggleRetainsCancellationCause prevents an online restore from disguising an interrupted request.
func TestRapidToggleRetainsCancellationCause(t *testing.T) {
	p := &Policy{}
	ctx, release, err := p.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	p.SetOffline(true)
	p.SetOffline(false)
	if !errors.Is(context.Cause(ctx), ErrOffline) {
		t.Fatal("mode cancellation lost its cause")
	}
}
