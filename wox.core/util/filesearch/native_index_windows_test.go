//go:build windows

package filesearch

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"wox/util/filesearchservice"
	"wox/util/filesearchservice/protocol"
)

// emitNativeIndexFixture sends the same bounded fact frames as the service.
func emitNativeIndexFixture(emit filesearchservice.FactSink, root, name string) error {
	boundary, _ := protocol.EncodeBoundary(protocol.Boundary{Volume: root, RootID: 5, JournalID: 10, NextUSN: 100})
	if err := emit(protocol.Begin, boundary); err != nil {
		return err
	}
	entries, _ := protocol.AppendEntry(nil, protocol.Entry{ID: 10, ParentID: 5, Name: name, USN: 90})
	if err := emit(protocol.Records, entries); err != nil {
		return err
	}
	return emit(protocol.End, boundary)
}

// awaitNativeMatches waits for an observable publication, not a worker sleep.
func awaitNativeMatches(t *testing.T, index *ntfsNameIndex, query string, count int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		results, err := index.Search(ctx, SearchQuery{Raw: query}, 50)
		if err == nil && len(results) == count {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("query %q: got %+v, error %v, want %d", query, results, err, count)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestNativeIndexSerialBuildsAndRestoresWithoutEnumeration(t *testing.T) {
	dir := t.TempDir()
	started := make(chan string, 2)
	release := make(chan struct{}, 2)
	var active, maximum atomic.Int32
	stream := func(ctx context.Context, kind protocol.Kind, boundary protocol.Boundary, emit filesearchservice.FactSink) error {
		if kind != protocol.Enumerate {
			<-ctx.Done()
			return ctx.Err()
		}
		current := active.Add(1)
		defer active.Add(-1)
		for old := maximum.Load(); current > old && !maximum.CompareAndSwap(old, current); old = maximum.Load() {
		}
		started <- boundary.Volume
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-release:
		}
		return emitNativeIndexFixture(emit, boundary.Volume, "项目报告.txt")
	}
	index, err := newNTFSNameIndex(context.Background(), dir, []string{`C:\`, `D:\`}, stream, nativeIndexCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	for n := 0; n < 2; n++ {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("build did not start")
		}
		release <- struct{}{}
	}
	awaitNativeMatches(t, index, "xmbg", 2)
	if maximum.Load() != 1 {
		t.Fatalf("concurrent builds=%d", maximum.Load())
	}
	if err := index.Close(); err != nil {
		t.Fatal(err)
	}
	var enumerations atomic.Int32
	restored, err := newNTFSNameIndex(context.Background(), dir, []string{`C:\`, `D:\`}, func(ctx context.Context, kind protocol.Kind, boundary protocol.Boundary, emit filesearchservice.FactSink) error {
		if kind == protocol.Enumerate {
			enumerations.Add(1)
			return errors.New("unexpected enumeration")
		}
		<-ctx.Done()
		return ctx.Err()
	}, nativeIndexCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	awaitNativeMatches(t, restored, "xmbg", 2)
	_, stats, err := restored.Status(context.Background())
	if err != nil || stats.FileCount != 2 || stats.RootCount != 2 || stats.DiskBytes == 0 {
		t.Fatalf("stats=%+v error=%v", stats, err)
	}
	closed := make(chan error, 1)
	go func() { closed <- restored.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("close did not cancel waiting subscriptions")
	}
	if enumerations.Load() != 0 {
		t.Fatal("restart enumerated existing volumes")
	}
}

// TestNativeDiagnosticsUseVolumeIndex verifies that empty configured roots do
// not hide the native index's files, volume roots, disk size or feed cursors.
func TestNativeDiagnosticsUseVolumeIndex(t *testing.T) {
	index, err := newNTFSNameIndex(context.Background(), t.TempDir(), []string{`C:\`}, func(ctx context.Context, kind protocol.Kind, boundary protocol.Boundary, emit filesearchservice.FactSink) error {
		if kind == protocol.Enumerate {
			return emitNativeIndexFixture(emit, boundary.Volume, "native.txt")
		}
		<-ctx.Done()
		return ctx.Err()
	}, nativeIndexCallbacks{})
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	awaitNativeMatches(t, index, "native", 1)
	engine := &Engine{nativeIndex: index}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	diagnostics, err := engine.GetDiagnostics(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !diagnostics.Index.CountsAvailable || diagnostics.Index.FileCount != 1 || diagnostics.Index.DiskBytes == 0 || diagnostics.RootCount != 1 || len(diagnostics.Roots) != 1 || diagnostics.Roots[0].FeedType != RootFeedTypeUSN || diagnostics.Roots[0].FeedCursor == "" {
		t.Fatalf("native diagnostic view: %+v", diagnostics)
	}
	stats, err := engine.GetIndexStats(ctx)
	if err != nil || stats.FileCount != diagnostics.Index.FileCount || stats.EntryCount != diagnostics.Index.EntryCount {
		t.Fatalf("Settings disagrees with diagnostics: %+v, %v", stats, err)
	}
}
