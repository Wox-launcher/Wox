//go:build windows

package filesearchservice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// startManagementTestServer isolates the pipe and joins the server on cleanup.
func startManagementTestServer(t *testing.T, handler RequestHandler) string {
	t.Helper()
	name := fmt.Sprintf(`\\.\pipe\WoxManagement-%d-%d`, os.Getpid(), time.Now().UnixNano())
	owner, err := currentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- servePipe(ctx, name, owner, handler) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Error(err)
			}
		case <-time.After(2 * time.Second):
			t.Error("management pipe did not stop")
		}
	})
	return name
}

func TestHealthHandshakeOverNamedPipe(t *testing.T) {
	name := startManagementTestServer(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	response, err := healthPipe(ctx, name)
	if err != nil || response.Protocol != ProtocolVersion || response.Version != EmbeddedVersion {
		t.Fatalf("health=%+v error=%v", response, err)
	}
	if _, err := callPipe(ctx, name, Request{Method: "search"}); err == nil {
		t.Fatal("management pipe accepted a filename query")
	}
}

func TestHealthRemainsAvailableDuringUpdate(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	acknowledged := make(chan struct{})
	name := startManagementTestServer(t, func(request Request) (func(), error) {
		if request.Method != "update" || request.CandidatePath != "candidate.exe" {
			return nil, errors.New("invalid request")
		}
		close(started)
		<-release
		return func() { close(acknowledged) }, nil
	})
	// Cleanup releases the handler before joining the server, including failures.
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := callPipe(ctx, name, Request{Method: "update", CandidatePath: "candidate.exe"})
		done <- err
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if _, err := healthPipe(ctx, name); err != nil {
		t.Fatalf("health during update: %v", err)
	}
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case <-acknowledged:
	case <-ctx.Done():
		t.Fatal("update completion callback was lost")
	}
}

func TestDecodeResponseRejectsOldProtocol(t *testing.T) {
	response, err := decodeResponse(strings.NewReader(`{"ok":true,"protocol":2,"version":"2.8.2"}`))
	if !errors.Is(err, ErrProtocolMismatch) || response.Protocol != 2 {
		t.Fatalf("response=%+v error=%v", response, err)
	}
}
