//go:build windows

package filesearch

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"wox/util"
	"wox/util/filesearchservice"
	"wox/util/filesearchservice/protocol"
)

// TestNativeIndexIncrementalCountNotifications exercises the same engine status
// subscription used by the toolbar, with content extraction explicitly disabled.
func TestNativeIndexIncrementalCountNotifications(t *testing.T) {
	ctx := context.Background()
	batches := make(chan []protocol.Entry)
	counts := make(chan int64, 16)
	ready := make(chan struct{})
	var engine *Engine
	var contentCalls atomic.Int32
	stream := func(ctx context.Context, kind protocol.Kind, boundary protocol.Boundary, emit filesearchservice.FactSink) error {
		if kind == protocol.Enumerate {
			return emitNativeIndexFixture(emit, boundary.Volume, "initial.txt")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case entries := <-batches:
			for _, entry := range entries {
				boundary.NextUSN++
				entry.USN = boundary.NextUSN
				payload, err := protocol.AppendEntry(nil, entry)
				if err != nil {
					return err
				}
				if err := emit(protocol.Records, payload); err != nil {
					return err
				}
			}
			payload, err := protocol.EncodeBoundary(boundary)
			if err != nil {
				return err
			}
			return emit(protocol.Checkpoint, payload)
		}
	}
	index, err := newNTFSNameIndex(ctx, t.TempDir(), []string{`C:\`}, stream, nativeIndexCallbacks{
		status: func() {
			select {
			case <-ready:
				engine.notifyStatusChanged(ctx)
			default:
			}
		},
		wantsContent: func() bool { return false },
		content:      func(context.Context, []ContentHookNotification) { contentCalls.Add(1) },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	engine = &Engine{nativeIndex: index, statusListeners: util.NewHashMap[string, func(StatusSnapshot)]()}
	engine.OnStatusChanged(func(status StatusSnapshot) {
		if status.IsIndexing {
			return
		}
		stats, err := engine.GetIndexStats(ctx)
		if err != nil {
			t.Error(err)
			return
		}
		counts <- stats.FileCount
	})
	close(ready)
	awaitNativeMatches(t, index, "initial", 1)
	// Wait for the worker to reach subscription, beyond full-build callbacks.
	for _, step := range []struct {
		entries []protocol.Entry
		want    int64
	}{
		{[]protocol.Entry{{ID: 12, ParentID: 5, Name: "first.txt"}, {ID: 13, ParentID: 5, Name: "second.txt"}, {ID: 14, ParentID: 5, Name: "folder", Flags: protocol.Directory}}, 3},
		{[]protocol.Entry{{ID: 12, ParentID: 5, Name: "first.txt", Flags: protocol.Deleted}}, 2},
	} {
		select {
		case batches <- step.entries:
		case <-time.After(3 * time.Second):
			t.Fatal("native index did not subscribe for changes")
		}
		timeout := time.After(3 * time.Second)
		updated := false
		for !updated {
			select {
			case count := <-counts:
				updated = count == step.want
			case <-timeout:
				t.Fatalf("no automatic count update to %d", step.want)
			}
		}
	}
	// The full-build replacement hook is existing behavior; incrementals must
	// not require content notifications in order to publish filename statistics.
	if contentCalls.Load() > 1 {
		t.Fatal("incremental count updates enabled content extraction")
	}
}

// TestNativeIndexCoalescesStatusChanges also checks that changes arriving during
// a slow consumer are delivered in the next window, and shutdown joins the worker.
func TestNativeIndexCoalescesStatusChanges(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	updates, release := make(chan struct{}, 2), make(chan struct{}, 2)
	index := &ntfsNameIndex{changes: make(chan struct{}, 1), notify: func() {
		updates <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
	}}
	for range 1000 {
		index.queueStatusChange()
	}
	index.workers.Add(1)
	go index.publishStatusChanges(ctx)
	defer func() {
		cancel()
		index.workers.Wait()
	}()
	for round := range 2 {
		select {
		case <-updates:
		case <-time.After(3 * time.Second):
			t.Fatalf("missing coalesced update %d", round)
		}
		if len(index.changes) != 0 {
			t.Fatal("burst was not consumed by one update")
		}
		if round == 0 {
			for range 1000 {
				index.queueStatusChange()
			}
		}
		release <- struct{}{}
	}
	cancel()
	index.workers.Wait()
	index.queueStatusChange()
	if len(updates) != 0 {
		t.Fatal("status worker emitted after shutdown")
	}
}

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

// TestNativeIndexReportsServiceCoverage verifies that consumers leave the service
// on transport failure, rejoin after replay, and leave again during shutdown.
func TestNativeIndexReportsServiceCoverage(t *testing.T) {
	states := make(chan bool, 8)
	var reads atomic.Int32
	stream := func(ctx context.Context, kind protocol.Kind, boundary protocol.Boundary, emit filesearchservice.FactSink) error {
		if kind == protocol.Enumerate {
			return emitNativeIndexFixture(emit, boundary.Volume, "app.lnk")
		}
		switch reads.Add(1) {
		case 1:
			return errors.New("service disconnected")
		case 2:
			payload, _ := protocol.EncodeBoundary(boundary)
			return emit(protocol.Checkpoint, payload)
		default:
			<-ctx.Done()
			return ctx.Err()
		}
	}
	index, err := newNTFSNameIndex(context.Background(), t.TempDir(), []string{`C:\`}, stream, nativeIndexCallbacks{
		availability: func(_ string, available bool) { states <- available },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	for _, want := range []bool{false, true, false, true} {
		select {
		case got := <-states:
			if got != want {
				t.Fatalf("coverage=%t, want %t", got, want)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("missing coverage transition to %t", want)
		}
	}
	if err := index.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case available := <-states:
		if available {
			t.Fatal("source remained available after close")
		}
	default:
		t.Fatal("close did not withdraw coverage")
	}
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
	db, ctx := openTestFileSearchDB(t)
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
	index, err := newNTFSNameIndex(ctx, dir, []string{`C:\`, `D:\`}, stream, nativeIndexCallbacks{
		loadElapsed: db.lastNativeFullIndexElapsedMs, saveElapsed: db.setNativeFullIndexElapsedMs,
	})
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
	var elapsed int64
	for _, root := range []string{`C:\`, `D:\`} {
		value, err := db.lastNativeFullIndexElapsedMs(ctx, root)
		if err != nil || value <= 0 {
			t.Fatalf("completed duration for %s: %d, %v", root, value, err)
		}
		elapsed += value
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = NewFileSearchDB(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var enumerations atomic.Int32
	restored, err := newNTFSNameIndex(context.Background(), dir, []string{`C:\`, `D:\`}, func(ctx context.Context, kind protocol.Kind, boundary protocol.Boundary, emit filesearchservice.FactSink) error {
		if kind == protocol.Enumerate {
			enumerations.Add(1)
			return errors.New("unexpected enumeration")
		}
		<-ctx.Done()
		return ctx.Err()
	}, nativeIndexCallbacks{loadElapsed: db.lastNativeFullIndexElapsedMs, saveElapsed: db.setNativeFullIndexElapsedMs})
	if err != nil {
		t.Fatal(err)
	}
	awaitNativeMatches(t, restored, "xmbg", 2)
	_, stats, err := restored.Status(context.Background())
	if err != nil || stats.FileCount != 2 || stats.RootCount != 2 || stats.DiskBytes == 0 || stats.LastElapsedMs != elapsed {
		t.Fatalf("stats=%+v error=%v", stats, err)
	}
	for _, shard := range restored.directory.shards {
		if err := shard.store.Compact(ctx); err != nil {
			t.Fatal(err)
		}
	}
	stats, err = (&Engine{nativeIndex: restored}).GetIndexStats(ctx)
	if err != nil || stats.LastElapsedMs != elapsed {
		t.Fatalf("compaction replaced Settings build duration: %+v, %v", stats, err)
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

// TestNativeIndexDurationKeepsLastSuccess covers live progress and failed retries.
func TestNativeIndexDurationKeepsLastSuccess(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	const root = `C:\`
	if err := db.setNativeFullIndexElapsedMs(ctx, root, 6000); err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-1500 * time.Millisecond)
	index := &ntfsNameIndex{
		directory:  newDirectoryIndex(t.TempDir()),
		runStarted: started,
		states:     map[string]nativeVolumeState{root: {building: true, started: started, elapsed: 6000}},
		callbacks:  nativeIndexCallbacks{saveElapsed: db.setNativeFullIndexElapsedMs},
	}
	status, stats, err := index.Status(ctx)
	if err != nil || !stats.IsIndexing || stats.LastElapsedMs < 1500 || stats.LastElapsedMs-status.ActiveRunElapsedMs > 10 {
		t.Fatalf("live duration: %+v, %+v, %v", status, stats, err)
	}
	index.setState(ctx, root, false, errors.New("enumeration failed"))
	_, stats, err = index.Status(ctx)
	saved, loadErr := db.lastNativeFullIndexElapsedMs(ctx, root)
	if err != nil || loadErr != nil || stats.LastElapsedMs != 6000 || saved != 6000 {
		t.Fatalf("failed build replaced last success: stats=%+v saved=%d errors=%v/%v", stats, saved, err, loadErr)
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
