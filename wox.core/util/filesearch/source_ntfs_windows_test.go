//go:build windows

package filesearch

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"wox/util/filesearch/column"
	"wox/util/filesearchservice"
	"wox/util/filesearchservice/protocol"
)

// emitFactTestBoundary keeps the replay tests focused on observable generations.
func emitFactTestBoundary(emit filesearchservice.FactSink, kind protocol.Kind, cursor int64) error {
	data, err := protocol.EncodeBoundary(protocol.Boundary{Volume: `C:\`, RootID: 5, JournalID: 10, NextUSN: cursor})
	if err != nil {
		return err
	}
	return emit(kind, data)
}

func TestNTFSBuildCatchesUpWithoutOverwritingNewerMFTRecords(t *testing.T) {
	store, err := column.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	source := ntfsSource{root: `C:\`, store: store, stream: func(ctx context.Context, kind protocol.Kind, boundary protocol.Boundary, emit filesearchservice.FactSink) error {
		var entries []protocol.Entry
		if kind == protocol.Enumerate {
			if err := emitFactTestBoundary(emit, protocol.Begin, 100); err != nil {
				return err
			}
			entries = []protocol.Entry{
				{ID: 10, ParentID: 5, Name: "new.txt", USN: 120},
				{ID: 11, ParentID: 5, Name: "removed.txt", USN: 90},
			}
		} else {
			if boundary.NextUSN != 100 {
				t.Fatalf("replay starts at %d", boundary.NextUSN)
			}
			entries = []protocol.Entry{
				{ID: 10, ParentID: 5, Name: "old.txt", USN: 105, Flags: protocol.Deleted},
				{ID: 10, ParentID: 5, Name: "intermediate.txt", USN: 106},
				{ID: 11, ParentID: 5, Name: "removed.txt", USN: 110, Flags: protocol.Deleted},
				{ID: 12, ParentID: 5, Name: "created.txt", USN: 125},
			}
		}
		var batch []byte
		for _, entry := range entries {
			var err error
			batch, err = protocol.AppendEntry(batch, entry)
			if err != nil {
				return err
			}
		}
		if err := emit(protocol.Records, batch); err != nil {
			return err
		}
		if kind == protocol.Enumerate {
			return emitFactTestBoundary(emit, protocol.End, 130)
		}
		return emitFactTestBoundary(emit, protocol.Checkpoint, 135)
	}}
	if err := source.build(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.Read(func(view column.View) error {
		if record, ok := view.Lookup(10); !ok || record.Name != "new.txt" {
			t.Errorf("newest MFT record lost: %+v", record)
		}
		if _, ok := view.Lookup(11); ok {
			t.Error("deleted record survived catch-up")
		}
		if _, ok := view.Lookup(12); !ok {
			t.Error("created record missing")
		}
		var cursor protocol.Boundary
		if err := json.Unmarshal([]byte(view.Cursor), &cursor); err != nil {
			return err
		}
		if cursor.NextUSN != 135 {
			t.Errorf("cursor=%d", cursor.NextUSN)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	revision := store.Revision()
	source.stream = func(context.Context, protocol.Kind, protocol.Boundary, filesearchservice.FactSink) error {
		return filesearchservice.ErrFactsReset
	}
	if err := source.build(context.Background()); !errors.Is(err, filesearchservice.ErrFactsReset) {
		t.Fatal(err)
	}
	if store.Revision() != revision {
		t.Fatal("failed rebuild replaced the searchable generation")
	}
}

func TestNTFSInterruptedReplayKeepsOldCursor(t *testing.T) {
	dir := t.TempDir()
	store, err := column.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	builder := column.NewBuilder(`C:\`, 5)
	if err := builder.Add(column.Record{ID: 10, ParentID: 5, Name: "old.txt", Version: 90}); err != nil {
		t.Fatal(err)
	}
	base, err := builder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	cursor, _ := json.Marshal(protocol.Boundary{Volume: `C:\`, RootID: 5, JournalID: 10, NextUSN: 100})
	if err := store.Publish(base, string(cursor), "ntfs-v3", 0); err != nil {
		t.Fatal(err)
	}
	interrupted := errors.New("connection lost")
	source := ntfsSource{root: `C:\`, store: store, stream: func(ctx context.Context, kind protocol.Kind, boundary protocol.Boundary, emit filesearchservice.FactSink) error {
		batch, _ := protocol.AppendEntry(nil, protocol.Entry{ID: 10, ParentID: 5, Name: "new.txt", USN: 110})
		if err := emit(protocol.Records, batch); err != nil {
			return err
		}
		return interrupted
	}}
	if err := source.replay(context.Background(), false); !errors.Is(err, interrupted) {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = column.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Read(func(view column.View) error {
		if view.Cursor != string(cursor) {
			t.Error("cursor advanced without a checkpoint")
		}
		if record, ok := view.Lookup(10); !ok || record.Name != "new.txt" {
			t.Error("durable batch was lost")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestNTFSNoopReplayDoesNotWriteJournal prevents an idle index from responding
// to its own journal writes with another write and an endless stream of USNs.
func TestNTFSNoopReplayDoesNotWriteJournal(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := column.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	builder := column.NewBuilder(`C:\`, 5)
	if err := builder.Add(column.Record{ID: 10, ParentID: 5, Name: "index.log", Version: 90}); err != nil {
		t.Fatal(err)
	}
	base, err := builder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	cursor, _ := json.Marshal(protocol.Boundary{Volume: `C:\`, RootID: 5, JournalID: 10, NextUSN: 100})
	if err := store.Publish(base, string(cursor), "ntfs-v3", 0); err != nil {
		t.Fatal(err)
	}
	source := ntfsSource{root: `C:\`, store: store}
	for iteration := 0; iteration < 3; iteration++ {
		before := store.Revision()
		source.stream = func(ctx context.Context, kind protocol.Kind, boundary protocol.Boundary, emit filesearchservice.FactSink) error {
			if want := int64(100 + iteration*10); boundary.NextUSN != want {
				t.Fatalf("cursor=%d, want %d", boundary.NextUSN, want)
			}
			batch, _ := protocol.AppendEntry(nil, protocol.Entry{ID: 10, ParentID: 5, Name: "index.log", USN: boundary.NextUSN + 1})
			if err := emit(protocol.Records, batch); err != nil {
				return err
			}
			return emitFactTestBoundary(emit, protocol.Checkpoint, boundary.NextUSN+10)
		}
		if err := source.replay(ctx, false); err != nil {
			t.Fatal(err)
		}
		if store.Revision() != before {
			t.Fatal("metadata-only replay wrote the index")
		}
	}
	source.stream = func(ctx context.Context, kind protocol.Kind, boundary protocol.Boundary, emit filesearchservice.FactSink) error {
		if boundary.NextUSN != 130 {
			t.Fatalf("volatile cursor=%d", boundary.NextUSN)
		}
		// Repeated IDs in one frame must be compared with their pending state;
		// returning to the original name is a real change after the rename.
		var batch []byte
		for _, entry := range []protocol.Entry{{ID: 10, ParentID: 5, Name: "renamed.log", USN: 131}, {ID: 10, ParentID: 5, Name: "index.log", USN: 132}, {ID: 11, ParentID: 5, Name: "created.txt", USN: 133}} {
			batch, _ = protocol.AppendEntry(batch, entry)
		}
		if err := emit(protocol.Records, batch); err != nil {
			return err
		}
		return emitFactTestBoundary(emit, protocol.Checkpoint, 140)
	}
	if err := source.replay(ctx, false); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = column.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Read(func(view column.View) error {
		var boundary protocol.Boundary
		if err := json.Unmarshal([]byte(view.Cursor), &boundary); err != nil {
			return err
		}
		if boundary.NextUSN != 140 {
			t.Fatalf("persisted cursor=%d", boundary.NextUSN)
		}
		if record, ok := view.Lookup(10); !ok || record.Name != "index.log" {
			t.Fatalf("rename sequence lost: %+v", record)
		}
		if _, ok := view.Lookup(11); !ok {
			t.Fatal("real change was not persisted")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestNTFSMetadataOnlyFactsStillNotifyContent separates filename no-op filtering
// from content changes; rewriting bytes does not necessarily rename a file.
func TestNTFSMetadataOnlyFactsStillNotifyContent(t *testing.T) {
	store, err := column.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	builder := column.NewBuilder(`C:\`, 5)
	if err := builder.Add(column.Record{ID: 10, ParentID: 5, Name: "body.txt", Version: 90}); err != nil {
		t.Fatal(err)
	}
	base, err := builder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	cursor, _ := json.Marshal(protocol.Boundary{Volume: `C:\`, RootID: 5, JournalID: 10, NextUSN: 100})
	if err := store.Publish(base, string(cursor), "ntfs-v3", 0); err != nil {
		t.Fatal(err)
	}
	var received []ContentHookNotification
	source := ntfsSource{root: `C:\`, store: store, notify: func(_ context.Context, notifications []ContentHookNotification) {
		received = append(received, notifications...)
	}, stream: func(ctx context.Context, kind protocol.Kind, boundary protocol.Boundary, emit filesearchservice.FactSink) error {
		batch, _ := protocol.AppendEntry(nil, protocol.Entry{ID: 10, ParentID: 5, Name: "body.txt", USN: 101})
		if err := emit(protocol.Records, batch); err != nil {
			return err
		}
		return emitFactTestBoundary(emit, protocol.Checkpoint, 110)
	}}
	revision := store.Revision()
	if err := source.replay(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if store.Revision() != revision {
		t.Fatal("metadata event wrote filename journal")
	}
	if len(received) != 1 || received[0].Kind != ContentHookKindUpsert || received[0].Path != `C:\body.txt` {
		t.Fatalf("content notifications=%+v", received)
	}
}

// TestNTFSDirectoryRenameAndDeleteNotifyBothScopes verifies that content can
// remove old descendant paths and discover the same IDs at their new paths.
func TestNTFSDirectoryRenameAndDeleteNotifyBothScopes(t *testing.T) {
	store, err := column.OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	builder := column.NewBuilder(`C:\`, 5)
	for _, record := range []column.Record{{ID: 10, ParentID: 5, Name: "old", Flags: column.Directory, Version: 90}, {ID: 11, ParentID: 10, Name: "body.txt", Version: 90}} {
		if err := builder.Add(record); err != nil {
			t.Fatal(err)
		}
	}
	base, err := builder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	cursor, _ := json.Marshal(protocol.Boundary{Volume: `C:\`, RootID: 5, JournalID: 10, NextUSN: 100})
	if err := store.Publish(base, string(cursor), "ntfs-v3", 0); err != nil {
		t.Fatal(err)
	}
	var received []ContentHookNotification
	source := ntfsSource{root: `C:\`, store: store, notify: func(_ context.Context, notifications []ContentHookNotification) {
		received = append(received, notifications...)
	}}
	for _, deleted := range []bool{false, true} {
		source.stream = func(ctx context.Context, kind protocol.Kind, boundary protocol.Boundary, emit filesearchservice.FactSink) error {
			flags := protocol.Directory
			if deleted {
				flags |= protocol.Deleted
			}
			batch, _ := protocol.AppendEntry(nil, protocol.Entry{ID: 10, ParentID: 5, Name: "new", Flags: flags, USN: boundary.NextUSN + 1})
			if err := emit(protocol.Records, batch); err != nil {
				return err
			}
			return emitFactTestBoundary(emit, protocol.Checkpoint, boundary.NextUSN+10)
		}
		received = nil
		if err := source.replay(context.Background(), false); err != nil {
			t.Fatal(err)
		}
		want := []string{`C:\old`, `C:\new`}
		if deleted {
			want = []string{`C:\new`}
		}
		if len(received) != len(want) {
			t.Fatalf("notifications=%+v", received)
		}
		for i, scope := range want {
			if received[i].Kind != ContentHookKindScopeReplaced || received[i].ScopePath != scope {
				t.Fatalf("notifications=%+v", received)
			}
		}
	}
}
