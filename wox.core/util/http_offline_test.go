package util

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"wox/network"
)

type offlineDownloadTransport func(*http.Request) (*http.Response, error)

func (f offlineDownloadTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestOfflineDownloadPreservesCompleteFile cancels after headers and checks atomic destination handling.
func TestOfflineDownloadPreservesCompleteFile(t *testing.T) {
	network.Default.SetOffline(false)
	defer network.Default.SetOffline(false)
	dir := t.TempDir()
	dest := filepath.Join(dir, "model.bin")
	if err := os.WriteFile(dest, []byte("complete"), 0600); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	reader, writer := io.Pipe()
	client := &http.Client{Transport: network.Wrap(offlineDownloadTransport(func(r *http.Request) (*http.Response, error) {
		go func() {
			_, _ = writer.Write([]byte("partial"))
			close(started)
			<-r.Context().Done()
			_ = writer.CloseWithError(r.Context().Err())
		}()
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: reader}, nil
	}))}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com/model", nil)
	finished := make(chan error, 1)
	go func() { finished <- httpDownloadWithClient(req.Context(), req, dest, nil, client, false) }()
	<-started
	network.Default.SetOffline(true)
	if err := <-finished; !errors.Is(err, network.ErrOffline) {
		t.Fatalf("download cancellation: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil || string(data) != "complete" {
		t.Fatalf("complete file lost: %q %v", data, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatal("temporary download survived cancellation")
	}
}
