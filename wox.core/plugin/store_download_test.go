package plugin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDownloadPluginFileTimesOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("partial plugin"))
		writer.(http.Flusher).Flush()
		<-request.Context().Done()
	}))
	defer server.Close()

	dest := filepath.Join(t.TempDir(), "plugin.zip")
	err := downloadPluginFile(context.Background(), server.URL, dest, 50*time.Millisecond, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if !strings.Contains(err.Error(), "plugin download timed out after 50ms") {
		t.Fatalf("expected explicit timeout error, got %v", err)
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Fatalf("expected incomplete download to be removed, stat error: %v", statErr)
	}
}

func TestDownloadPluginFilePreservesParentCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		<-request.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := downloadPluginFile(ctx, server.URL, filepath.Join(t.TempDir(), "plugin.zip"), time.Minute, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	if strings.Contains(err.Error(), "timed out") {
		t.Fatalf("parent cancellation should not be reported as a timeout: %v", err)
	}
}
