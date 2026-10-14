//go:build windows

package filesearch

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"wox/util/filesearch/column"
	"wox/util/filesearchservice"
	"wox/util/filesearchservice/protocol"
)

// TestNativeBuildProgressBeforePublication covers waiting, multiple streamed
// batches, finalization and ticker shutdown through the real Engine status path.
func TestNativeBuildProgressBeforePublication(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	index := &ntfsNameIndex{directory: newDirectoryIndex(t.TempDir()), states: make(map[string]nativeVolumeState)}
	defer index.directory.close()
	const root = `C:\`
	if err := index.directory.attach(RootRecord{ID: root, Path: root}); err != nil {
		t.Fatal(err)
	}
	index.states[root] = nativeVolumeState{pending: true}
	engine := &Engine{nativeIndex: index}
	events := make(chan StatusSnapshot, 64)
	var notifications atomic.Int32
	index.notify = func() {
		status, err := engine.GetStatus(ctx)
		if err != nil {
			return
		}
		notifications.Add(1)
		select {
		case events <- status:
		default:
		}
	}
	first, second, end, finalize := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	wait := func(gate <-chan struct{}) error {
		select {
		case <-gate:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	source := &ntfsSource{
		root: root, store: index.directory.shards[root].store,
		progress: func(progress nativeBuildProgress) {
			index.updateProgress(root, progress)
			if progress.finalizing {
				_ = wait(finalize)
			}
		},
		stream: func(_ context.Context, kind protocol.Kind, _ protocol.Boundary, emit filesearchservice.FactSink) error {
			if kind != protocol.Enumerate {
				return errors.New("unexpected replay")
			}
			boundary, _ := protocol.EncodeBoundary(protocol.Boundary{Volume: root, RootID: 5, JournalID: 10, NextUSN: 100})
			if err := emit(protocol.Begin, boundary); err != nil {
				return err
			}
			for number, gate := range []<-chan struct{}{first, second} {
				if err := wait(gate); err != nil {
					return err
				}
				payload, _ := protocol.AppendEntry(nil, protocol.Entry{ID: uint64(10 + number), ParentID: 5, Name: "file.txt", USN: 90})
				if err := emit(protocol.Records, payload); err != nil {
					return err
				}
			}
			if err := wait(end); err != nil {
				return err
			}
			return emit(protocol.End, boundary)
		},
	}
	result, finished := make(chan error, 1), make(chan struct{})
	go func() { defer close(finished); result <- index.buildVolume(ctx, root, source) }()
	defer func() { cancel(); <-finished }()
	await := func(predicate func(StatusSnapshot) bool) StatusSnapshot {
		t.Helper()
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		for {
			select {
			case status := <-events:
				if predicate(status) {
					return status
				}
			case <-timer.C:
				t.Fatal("missing build progress notification")
				return StatusSnapshot{}
			}
		}
	}
	await(func(status StatusSnapshot) bool { return status.IsIndexing && status.ActiveRunFileCount == 0 })
	waiting := await(func(status StatusSnapshot) bool { return status.IsIndexing && status.ActiveRunElapsedMs >= 200 })
	close(first)
	one := await(func(status StatusSnapshot) bool { return status.ActiveRunFileCount == 1 })
	if one.ActiveRunElapsedMs < waiting.ActiveRunElapsedMs || one.ActiveRunEntryCount != 1 {
		t.Fatalf("first batch progress: %+v", one)
	}
	if err := source.store.Read(func(column.View) error { return nil }); !errors.Is(err, column.ErrNotReady) {
		t.Fatalf("progress must precede column publication: %v", err)
	}
	close(second)
	await(func(status StatusSnapshot) bool { return status.ActiveRunFileCount == 2 })
	close(end)
	saving := await(func(status StatusSnapshot) bool { return status.ActiveRunStatus == RunStatusFinalizing })
	await(func(status StatusSnapshot) bool {
		return status.ActiveRunStatus == RunStatusFinalizing && status.ActiveRunElapsedMs > saving.ActiveRunElapsedMs
	})
	close(finalize)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	await(func(status StatusSnapshot) bool { return !status.IsIndexing && status.ActiveRunFileCount == 2 })
	count := notifications.Load()
	time.Sleep(600 * time.Millisecond)
	if notifications.Load() != count {
		t.Fatal("build heartbeat kept running after completion")
	}
}

// TestNativeBuildProgressStopsOnCancel prevents an interrupted full scan from
// retaining a heartbeat or delaying shutdown while the fact stream is waiting.
func TestNativeBuildProgressStopsOnCancel(t *testing.T) {
	started := make(chan struct{})
	var notifications atomic.Int32
	index, err := newNTFSNameIndex(context.Background(), t.TempDir(), []string{`C:\`}, func(ctx context.Context, _ protocol.Kind, _ protocol.Boundary, _ filesearchservice.FactSink) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}, nativeIndexCallbacks{status: func() { notifications.Add(1) }})
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("build did not start")
	}
	closed := make(chan error, 1)
	go func() { closed <- index.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled build did not stop")
	}
	count := notifications.Load()
	time.Sleep(300 * time.Millisecond)
	if notifications.Load() != count {
		t.Fatal("cancelled build retained its progress heartbeat")
	}
}
