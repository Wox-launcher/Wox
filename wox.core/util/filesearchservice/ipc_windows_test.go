//go:build windows

package filesearchservice

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestHealthHandshakeOverNamedPipe(t *testing.T) {
	pipeName := fmt.Sprintf(`\\.\pipe\WoxFileIndexServiceTest-%d`, os.Getpid())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverErr := make(chan error, 1)
	ownerSID, err := currentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	wantStats := IndexStats{IsIndexing: true, EntryCount: 1250, FileCount: 1000, ElapsedMs: 80}
	go func() { serverErr <- servePipe(ctx, pipeName, ownerSID, nil, func() IndexStats { return wantStats }) }()

	healthCtx, healthCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer healthCancel()
	response, err := healthPipe(healthCtx, pipeName)
	if err != nil {
		t.Fatalf("health handshake: %v", err)
	}
	if response.Protocol != ProtocolVersion || response.Version != EmbeddedVersion {
		t.Fatalf("health response = %+v", response)
	}
	if response.IndexStats == nil || response.IndexStats.IsIndexing != wantStats.IsIndexing || response.IndexStats.EntryCount != wantStats.EntryCount || response.IndexStats.FileCount != wantStats.FileCount || response.IndexStats.ElapsedMs != wantStats.ElapsedMs {
		t.Fatalf("health index stats = %+v, want %+v", response.IndexStats, wantStats)
	}
}

func TestSnapshotStreamsEntriesOverNamedPipe(t *testing.T) {
	pipeName := fmt.Sprintf(`\\.\pipe\WoxFileIndexServiceSnapshotTest-%d`, os.Getpid())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ownerSID, err := currentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		_ = servePipe(ctx, pipeName, ownerSID, func(request Request, emit func(SnapshotEntry) error) (func(), error) {
			if request.RootPath != `C:\root` || len(request.RootPaths) != 1 || request.RootPaths[0] != request.RootPath {
				return nil, fmt.Errorf("roots=%q/%q", request.RootPath, request.RootPaths)
			}
			for index := 0; index < snapshotFrameBatchSize+1; index++ {
				if err := emit(SnapshotEntry{Path: fmt.Sprintf(`C:\root\%d`, index), IsDir: index == 0}); err != nil {
					return nil, err
				}
			}
			return nil, nil
		})
	}()

	requestCtx, requestCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer requestCancel()
	var entries []SnapshotEntry
	if err := snapshotPipe(requestCtx, pipeName, `C:\root`, func(entry SnapshotEntry) error {
		entries = append(entries, entry)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(entries) != snapshotFrameBatchSize+1 || !entries[0].IsDir || entries[len(entries)-1].Path != fmt.Sprintf(`C:\root\%d`, snapshotFrameBatchSize) {
		t.Fatalf("entries=%+v", entries)
	}
}

func TestHealthRemainsAvailableDuringSnapshot(t *testing.T) {
	pipeName := fmt.Sprintf(`\\.\pipe\WoxFileIndexServiceConcurrentTest-%d`, os.Getpid())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ownerSID, err := currentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = servePipe(ctx, pipeName, ownerSID, func(Request, func(SnapshotEntry) error) (func(), error) {
			close(started)
			<-release
			return nil, nil
		})
	}()
	requestCtx, requestCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer requestCancel()
	done := make(chan error, 1)
	go func() { done <- snapshotPipe(requestCtx, pipeName, `C:\`, nil) }()
	select {
	case <-started:
	case <-requestCtx.Done():
		t.Fatal(requestCtx.Err())
	}
	if _, err := healthPipe(requestCtx, pipeName); err != nil {
		t.Fatalf("health during snapshot: %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
