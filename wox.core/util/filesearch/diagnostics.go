package filesearch

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// DiagnosticSnapshot is intentionally broader than the user-facing status
// snapshot. The dev status command needs to answer "is the engine stuck before
// DB writes, inside the dirty queue, or inside a feed/root state" without adding
// another round of ad-hoc logging each time an incremental issue appears.
type DiagnosticSnapshot struct {
	CapturedAt time.Time

	Status StatusSnapshot

	RootCount            int
	UserVisibleRootCount int
	DefaultRootCount     int
	UserRootCount        int
	DynamicRootCount     int
	RootKindCounts       map[RootKind]int
	RootStatusCounts     map[RootStatus]int
	RootFeedTypeCounts   map[RootFeedType]int
	RootFeedStateCounts  map[RootFeedState]int
	Roots                []RootDiagnostic

	DirtyQueue DirtyQueueDiagnostics
	Index      IndexDiagnosticSnapshot
}

type RootDiagnostic struct {
	ID                  string
	Path                string
	Kind                RootKind
	Status              RootStatus
	FeedType            RootFeedType
	FeedState           RootFeedState
	FeedCursor          string
	LastReconcileAt     int64
	LastFullScanAt      int64
	ProgressCurrent     int64
	ProgressTotal       int64
	LastError           string
	DynamicParentRootID string
	PolicyRootPath      string
	PromotedAt          int64
	LastHotAt           int64
	CreatedAt           int64
	UpdatedAt           int64
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
	CountsAvailable        bool
	RootCount              int
	EntryCount             int64
	FileCount              int64
	BigramRowCount         int64
	FactBytesEstimate      int64
	FTSSourceBytesEstimate int64
	BigramBytesEstimate    int64
	TotalBytesEstimate     int64
	DBMainFileBytes        int64
	DBWALFileBytes         int64
	DBSHMFileBytes         int64
	DBTotalFileBytes       int64
	NameFTSVocab           int64
	PathFTSVocab           int64
	PinyinFullFTSVocab     int64
	InitialsFTSVocab       int64
	TopRoots               []IndexRootDiagnostic
	Error                  string
}

type IndexRootDiagnostic struct {
	RootID                 string
	Path                   string
	Docs                   int64
	BigramRows             int64
	FactBytesEstimate      int64
	FTSSourceBytesEstimate int64
	BigramBytesEstimate    int64
	TotalBytesEstimate     int64
}

func (e *Engine) GetDiagnostics(ctx context.Context) (DiagnosticSnapshot, error) {
	if e == nil {
		return DiagnosticSnapshot{}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

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
	diagnostics.UserVisibleRootCount = len(userVisibleRoots(roots))
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
		case RootKindDynamic:
			diagnostics.DynamicRootCount++
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
		diagnostics.Index = indexDiagnosticFromSQLiteSnapshot(snapshot)
	}

	return diagnostics, nil
}

func rootDiagnosticFromRecord(root RootRecord) RootDiagnostic {
	lastError := ""
	if root.LastError != nil {
		lastError = strings.TrimSpace(*root.LastError)
	}
	return RootDiagnostic{
		ID:                  root.ID,
		Path:                root.Path,
		Kind:                root.Kind,
		Status:              root.Status,
		FeedType:            root.FeedType,
		FeedState:           root.FeedState,
		FeedCursor:          root.FeedCursor,
		LastReconcileAt:     root.LastReconcileAt,
		LastFullScanAt:      root.LastFullScanAt,
		ProgressCurrent:     root.ProgressCurrent,
		ProgressTotal:       root.ProgressTotal,
		LastError:           lastError,
		DynamicParentRootID: root.DynamicParentRootID,
		PolicyRootPath:      root.PolicyRootPath,
		PromotedAt:          root.PromotedAt,
		LastHotAt:           root.LastHotAt,
		CreatedAt:           root.CreatedAt,
		UpdatedAt:           root.UpdatedAt,
	}
}

func indexDiagnosticFromSQLiteSnapshot(snapshot sqliteIndexSnapshot) IndexDiagnosticSnapshot {
	diagnostics := IndexDiagnosticSnapshot{
		CountsAvailable:        true,
		RootCount:              snapshot.RootCount,
		EntryCount:             snapshot.EntryCount,
		FileCount:              snapshot.FileCount,
		BigramRowCount:         snapshot.BigramRowCount,
		FactBytesEstimate:      snapshot.FactBytesEstimate,
		FTSSourceBytesEstimate: snapshot.FTSSourceBytesEstimate,
		BigramBytesEstimate:    snapshot.BigramBytesEstimate,
		TotalBytesEstimate:     snapshot.TotalBytesEstimate,
		DBMainFileBytes:        snapshot.DBMainFileBytes,
		DBWALFileBytes:         snapshot.DBWALFileBytes,
		DBSHMFileBytes:         snapshot.DBSHMFileBytes,
		DBTotalFileBytes:       snapshot.DBTotalFileBytes,
		NameFTSVocab:           snapshot.NameFTSVocab,
		PathFTSVocab:           snapshot.PathFTSVocab,
		PinyinFullFTSVocab:     snapshot.PinyinFullFTSVocab,
		InitialsFTSVocab:       snapshot.InitialsFTSVocab,
	}
	for _, root := range snapshot.TopRoots {
		diagnostics.TopRoots = append(diagnostics.TopRoots, IndexRootDiagnostic{
			RootID:                 root.RootID,
			Path:                   root.Path,
			Docs:                   root.Docs,
			BigramRows:             root.BigramRows,
			FactBytesEstimate:      root.FactBytesEstimate,
			FTSSourceBytesEstimate: root.FTSSourceBytesEstimate,
			BigramBytesEstimate:    root.BigramBytesEstimate,
			TotalBytesEstimate:     root.TotalBytesEstimate,
		})
	}
	return diagnostics
}
