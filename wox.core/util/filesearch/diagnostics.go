package filesearch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"wox/util/filesearch/column"
)

// DiagnosticSnapshot is intentionally broader than the user-facing status
// snapshot. The dev status command needs to answer "is the engine stuck before
// DB writes, inside the dirty queue, or inside a feed/root state" without adding
// another round of ad-hoc logging each time an incremental issue appears.
type DiagnosticSnapshot struct {
	CapturedAt time.Time

	Status StatusSnapshot

	RootCount           int
	DefaultRootCount    int
	UserRootCount       int
	RootKindCounts      map[RootKind]int
	RootStatusCounts    map[RootStatus]int
	RootFeedTypeCounts  map[RootFeedType]int
	RootFeedStateCounts map[RootFeedState]int
	Roots               []RootDiagnostic

	DirtyQueue DirtyQueueDiagnostics
	Index      IndexDiagnosticSnapshot
}

type RootDiagnostic struct {
	ID              string
	Path            string
	Kind            RootKind
	Status          RootStatus
	FeedType        RootFeedType
	FeedState       RootFeedState
	FeedCursor      string
	LastReconcileAt int64
	LastFullScanAt  int64
	ProgressCurrent int64
	ProgressTotal   int64
	LastError       string
	CreatedAt       int64
	UpdatedAt       int64
}

type DirtyQueueDiagnostics struct {
	PendingRootCount       int
	PendingRootSignalCount int
	PendingPathCount       int
	EarliestSignal         time.Time
	LatestSignal           time.Time
	CurrentDebounceWindow  time.Duration
	NextFlushIn            time.Duration
	LastDirtyRunElapsed    time.Duration
	Config                 DirtyQueueConfig
}

type IndexDiagnosticSnapshot struct {
	CountsAvailable          bool
	RootCount                int
	EntryCount, FileCount    int64
	DiskBytes, MetadataBytes int64
	TopRoots                 []IndexRootDiagnostic
	Error                    string
}

type IndexRootDiagnostic struct {
	RootID, Path    string
	Docs, DiskBytes int64
}

func (e *Engine) GetDiagnostics(ctx context.Context) (DiagnosticSnapshot, error) {
	if e == nil {
		return DiagnosticSnapshot{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	e.mu.RLock()
	if e.nativeIndex != nil {
		diagnostics, err := nativeIndexDiagnostics(ctx, e.nativeIndex)
		e.mu.RUnlock()
		return diagnostics, err
	}
	e.mu.RUnlock()

	status, err := e.GetStatus(ctx)
	if err != nil {
		return DiagnosticSnapshot{}, err
	}

	e.mu.RLock()
	if e.closed || e.db == nil {
		e.mu.RUnlock()
		return DiagnosticSnapshot{}, fmt.Errorf("filesearch engine closed")
	}
	db := e.db
	scanner := e.scanner
	e.mu.RUnlock()

	roots, err := db.ListRoots(ctx)
	if err != nil {
		return DiagnosticSnapshot{}, err
	}

	diagnostics := DiagnosticSnapshot{
		CapturedAt:          time.Now(),
		Status:              status,
		RootKindCounts:      map[RootKind]int{},
		RootStatusCounts:    map[RootStatus]int{},
		RootFeedTypeCounts:  map[RootFeedType]int{},
		RootFeedStateCounts: map[RootFeedState]int{},
	}
	// Feature addition: collect root and queue state in the engine instead of the
	// File Search plugin. The plugin should only format diagnostics; keeping the
	// scanner/database inspection here avoids future UI commands reaching into
	// private indexing internals when another incremental issue needs triage.
	diagnostics.RootCount = len(roots)
	for _, root := range roots {
		diagnostics.RootKindCounts[root.Kind]++
		diagnostics.RootStatusCounts[root.Status]++
		diagnostics.RootFeedTypeCounts[root.FeedType]++
		diagnostics.RootFeedStateCounts[root.FeedState]++
		switch root.Kind {
		case RootKindDefault:
			diagnostics.DefaultRootCount++
		case RootKindUser:
			diagnostics.UserRootCount++
		}
		diagnostics.Roots = append(diagnostics.Roots, rootDiagnosticFromRecord(root))
	}
	sort.Slice(diagnostics.Roots, func(i, j int) bool {
		return diagnostics.Roots[i].Path < diagnostics.Roots[j].Path
	})

	if scanner != nil {
		diagnostics.DirtyQueue = scanner.GetDirtyQueueDiagnostics(time.Now())
	}

	// Bug fix: `f status` must always surface the current persisted index volume.
	// The full SQLite snapshot also samples FTS vocab, size estimates, and top
	// roots, which can exceed the diagnostic timeout on a busy or large index.
	// Capture the cheap count query first so snapshot timeout remains a detail
	// instead of hiding the basic file/entry totals users need for triage.
	if fileCount, entryCount, err := db.SearchIndexCounts(ctx); err != nil {
		diagnostics.Index.Error = strings.TrimSpace(err.Error())
	} else {
		diagnostics.Index.CountsAvailable = true
		diagnostics.Index.RootCount = diagnostics.RootCount
		diagnostics.Index.FileCount = fileCount
		diagnostics.Index.EntryCount = entryCount
	}

	if snapshot, err := db.SearchIndexSnapshot(ctx); err != nil {
		if diagnostics.Index.Error == "" {
			diagnostics.Index.Error = strings.TrimSpace(err.Error())
		}
	} else {
		diagnostics.Index = indexDiagnosticFromColumnSnapshot(snapshot)
	}

	return diagnostics, nil
}

// nativeIndexDiagnostics reports the same volume-backed columns as search and
// Settings. Configured roots can be empty while the native index contains files.
func nativeIndexDiagnostics(ctx context.Context, index nativeNameIndex) (DiagnosticSnapshot, error) {
	status, stats, err := index.Status(ctx)
	if err != nil {
		return DiagnosticSnapshot{}, err
	}
	diagnostics := DiagnosticSnapshot{
		CapturedAt: time.Now(), Status: status,
		RootCount:           status.RootCount,
		DefaultRootCount:    status.RootCount,
		RootKindCounts:      map[RootKind]int{RootKindDefault: status.RootCount},
		RootStatusCounts:    map[RootStatus]int{},
		RootFeedTypeCounts:  map[RootFeedType]int{RootFeedTypeUSN: status.RootCount},
		RootFeedStateCounts: map[RootFeedState]int{},
		Index: IndexDiagnosticSnapshot{CountsAvailable: true, RootCount: stats.RootCount,
			EntryCount: stats.EntryCount, FileCount: stats.FileCount, DiskBytes: stats.DiskBytes, Error: stats.Error},
	}
	directory := index.directoryView()
	directory.mu.RLock()
	defer directory.mu.RUnlock()
	for _, shard := range directory.shards {
		root := RootDiagnostic{ID: shard.root.ID, Path: shard.root.Path, Kind: RootKindDefault,
			Status: RootStatusIdle, FeedType: RootFeedTypeUSN, FeedState: RootFeedStateReady}
		if status.IsIndexing && (status.ActiveRootPath == "" || normalizePath(status.ActiveRootPath) == normalizePath(root.Path)) {
			root.Status = RootStatusScanning
		}
		if normalizePath(status.ErrorRootPath) == normalizePath(root.Path) && status.LastError != "" {
			root.Status, root.FeedState, root.LastError = RootStatusError, RootFeedStateDegraded, status.LastError
		}
		if err := shard.store.Read(func(view column.View) error {
			root.FeedCursor = view.Cursor
			return nil
		}); errors.Is(err, column.ErrNotReady) {
			root.Status, root.FeedState = RootStatusPreparing, RootFeedStateUnavailable
		} else if err != nil {
			return DiagnosticSnapshot{}, err
		}
		diagnostics.Roots = append(diagnostics.Roots, root)
		diagnostics.RootStatusCounts[root.Status]++
		diagnostics.RootFeedStateCounts[root.FeedState]++
		_, entries, err := shard.store.Counts(ctx)
		if err != nil && !errors.Is(err, column.ErrNotReady) {
			return DiagnosticSnapshot{}, err
		}
		rootIndex := IndexRootDiagnostic{RootID: root.ID, Path: root.Path, Docs: entries}
		_ = filepath.WalkDir(directory.shardDirectory(root.ID), func(path string, entry os.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				rootIndex.DiskBytes += fileSizeOrZero(path)
			}
			return ctx.Err()
		})
		diagnostics.Index.TopRoots = append(diagnostics.Index.TopRoots, rootIndex)
	}
	sort.Slice(diagnostics.Roots, func(i, j int) bool { return diagnostics.Roots[i].Path < diagnostics.Roots[j].Path })
	sort.Slice(diagnostics.Index.TopRoots, func(i, j int) bool { return diagnostics.Index.TopRoots[i].Docs > diagnostics.Index.TopRoots[j].Docs })
	return diagnostics, nil
}

func rootDiagnosticFromRecord(root RootRecord) RootDiagnostic {
	lastError := ""
	if root.LastError != nil {
		lastError = strings.TrimSpace(*root.LastError)
	}
	return RootDiagnostic{
		ID:              root.ID,
		Path:            root.Path,
		Kind:            root.Kind,
		Status:          root.Status,
		FeedType:        root.FeedType,
		FeedState:       root.FeedState,
		FeedCursor:      root.FeedCursor,
		LastReconcileAt: root.LastReconcileAt,
		LastFullScanAt:  root.LastFullScanAt,
		ProgressCurrent: root.ProgressCurrent,
		ProgressTotal:   root.ProgressTotal,
		LastError:       lastError,
		CreatedAt:       root.CreatedAt,
		UpdatedAt:       root.UpdatedAt,
	}
}

func indexDiagnosticFromColumnSnapshot(snapshot columnIndexSnapshot) IndexDiagnosticSnapshot {
	diagnostics := IndexDiagnosticSnapshot{CountsAvailable: true, RootCount: snapshot.RootCount, EntryCount: snapshot.EntryCount, FileCount: snapshot.FileCount, DiskBytes: snapshot.DiskBytes, MetadataBytes: snapshot.MetadataBytes}
	for _, root := range snapshot.TopRoots {
		diagnostics.TopRoots = append(diagnostics.TopRoots, IndexRootDiagnostic{RootID: root.RootID, Path: root.Path, Docs: root.Docs, DiskBytes: root.DiskBytes})
	}
	return diagnostics
}
