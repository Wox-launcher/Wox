package filesearch

import "time"

type SearchQuery struct {
	Raw string
	// DisablePinyin lets callers mirror the global Wox pinyin setting while
	// preserving the historical default for internal tests and callers that do
	// not provide setting context.
	DisablePinyin bool
	wildcard      *wildcardQuery
	plan          *queryPlan
}

type StatusSnapshot struct {
	RootCount             int
	PreparingRootCount    int
	ScanningRootCount     int
	SyncingRootCount      int
	WritingRootCount      int
	FinalizingRootCount   int
	ErrorRootCount        int
	PendingDirtyRootCount int
	PendingDirtyPathCount int
	ProgressCurrent       int64
	ProgressTotal         int64
	ActiveRootStatus      RootStatus
	ActiveProgressCurrent int64
	ActiveProgressTotal   int64
	ActiveRootIndex       int
	ActiveRootTotal       int
	ActiveDiscoveredCount int64
	ActiveDirectoryIndex  int
	ActiveDirectoryTotal  int
	ActiveItemCurrent     int64
	ActiveItemTotal       int64
	// Root-local progress was no longer enough once one logical root could fan
	// out into many execution jobs, so these fields expose the active run state
	// without removing the existing root-centric compatibility data.
	ActiveRootPath     string
	ActiveRunStatus    RunStatus
	ActiveRunKind      RunKind
	ActiveJobKind      JobKind
	ActiveScopePath    string
	ActiveStage        RunStage
	RunProgressCurrent int64
	RunProgressTotal   int64
	// ActiveRunFileCount and ActiveRunEntryCount are live completed counts while
	// a run is executing, then final persisted counts on the completion summary.
	// Streaming full runs intentionally skip planner-side recursive counting, so
	// these values must come from the execution/write boundary instead of
	// EstimatedTotals.
	ActiveRunFileCount  int64
	ActiveRunEntryCount int64
	// ActiveRunElapsedMs is updated while a run is live and preserved on the
	// completion summary so toolbar consumers can show both live throughput and
	// the final elapsed time from the same end-to-end boundary.
	ActiveRunElapsedMs int64
	ErrorRootPath      string
	IsIndexing         bool
	IsInitialIndexing  bool
	LastError          string
}

type SearchResult struct {
	Path       string
	Name       string
	ParentPath string
	IsDir      bool
	// Refinement sorting needs indexed metadata in the result envelope so the
	// File Search plugin can sort already-recalled candidates without adding a
	// second database lookup for every visible row.
	Mtime int64
	Size  int64
	Score int64
}

type DirtySignalKind string

const (
	DirtySignalKindRoot DirtySignalKind = "root"
	DirtySignalKindPath DirtySignalKind = "path"
)

type DirtySignal struct {
	Kind          DirtySignalKind
	SemanticKind  ChangeSemanticKind
	RootID        string
	TraceID       string
	Path          string
	PathIsDir     bool
	PathTypeKnown bool
	At            time.Time
}

type ReconcileMode string

const (
	ReconcileModeSubtree ReconcileMode = "subtree"
	ReconcileModeRoot    ReconcileMode = "root"
	// ReconcileModeDirectDelta applies known file changes by exact path instead
	// of widening them to the parent directory. Directory and unknown-type
	// changes still use subtree/root modes because they own recursive deletes.
	ReconcileModeDirectDelta ReconcileMode = "direct_delta"
)

type ReconcileBatch struct {
	RootID         string
	TraceID        string
	Mode           ReconcileMode
	Paths          []string
	DirectDeltas   []PathDelta
	DirtyPathCount int
}

type PathDelta struct {
	Path          string
	SemanticKind  ChangeSemanticKind
	PathIsDir     bool
	PathTypeKnown bool
}

type RootFeedType string

const (
	RootFeedTypeFallback RootFeedType = "fallback"
	RootFeedTypeFSEvents RootFeedType = "fsevents"
	RootFeedTypeUSN      RootFeedType = "usn"
)

type RootFeedState string

const (
	RootFeedStateReady       RootFeedState = "ready"
	RootFeedStateDegraded    RootFeedState = "degraded"
	RootFeedStateUnavailable RootFeedState = "unavailable"
)

type RootKind string

const (
	RootKindDefault RootKind = "default"
	RootKindUser    RootKind = "user"
	// RootKindDynamic is an internal ownership boundary promoted from a hot
	// subdirectory. It stays hidden from user settings but lets the scanner
	// reconcile that subtree without rewriting the parent root's entries.
	RootKindDynamic RootKind = "dynamic"
)

type RootStatus string

const (
	RootStatusIdle       RootStatus = "idle"
	RootStatusPreparing  RootStatus = "preparing"
	RootStatusScanning   RootStatus = "scanning"
	RootStatusSyncing    RootStatus = "syncing"
	RootStatusWriting    RootStatus = "writing"
	RootStatusFinalizing RootStatus = "finalizing"
	RootStatusError      RootStatus = "error"
)

type ReplaceEntriesStage string

const (
	ReplaceEntriesStagePreparing  ReplaceEntriesStage = "preparing"
	ReplaceEntriesStageWriting    ReplaceEntriesStage = "writing"
	ReplaceEntriesStageFinalizing ReplaceEntriesStage = "finalizing"
)

type ReplaceEntriesProgress struct {
	Stage   ReplaceEntriesStage
	Current int64
	Total   int64
}

type TransientRootState struct {
	Root            RootRecord
	RootIndex       int
	RootTotal       int
	DiscoveredCount int64
	DirectoryIndex  int
	DirectoryTotal  int
	ItemCurrent     int64
	ItemTotal       int64
}

type TransientSyncState struct {
	Root             RootRecord
	RootIndex        int
	RootTotal        int
	Mode             ReconcileMode
	ScopeCount       int
	DirtyPathCount   int
	PendingRootCount int
	PendingPathCount int
}

type RootRecord struct {
	ID                  string
	Path                string
	Kind                RootKind
	Status              RootStatus
	FeedType            RootFeedType
	FeedCursor          string
	FeedState           RootFeedState
	LastReconcileAt     int64
	LastFullScanAt      int64
	ProgressCurrent     int64
	ProgressTotal       int64
	LastError           *string
	DynamicParentRootID string
	PolicyRootPath      string
	PromotedAt          int64
	LastHotAt           int64
	CreatedAt           int64
	UpdatedAt           int64
}

const RootProgressScale int64 = 1000

type EntryRecord struct {
	Path           string
	RootID         string
	ParentPath     string
	Name           string
	NormalizedName string
	NormalizedPath string
	PinyinFull     string
	PinyinInitials string
	IsDir          bool
	Mtime          int64
	Size           int64
	UpdatedAt      int64
}

type EntryUpdate struct {
	Old EntryRecord
	New EntryRecord
}

type EntryDeltaBatch struct {
	RootID        string
	PreviousCount int
	NextCount     int
	Added         []EntryRecord
	Updated       []EntryUpdate
	Removed       []EntryRecord
	ForceRebuild  bool
}

type DirectoryRecord struct {
	Path         string
	RootID       string
	ParentPath   string
	LastScanTime int64
	Exists       bool
}

type SubtreeSnapshotBatch struct {
	RootID      string
	ScopePath   string
	Directories []DirectoryRecord
	Entries     []EntryRecord
}

// JobApplyStats carries the entry counts observed at the write boundary. Full
// streaming runs intentionally skip planner-side recursive counting, so these
// numbers are the source of truth for live toolbar progress.
type JobApplyStats struct {
	EntryCount int64
	FileCount  int64
}

func jobApplyStatsFromBatch(batch SubtreeSnapshotBatch) JobApplyStats {
	stats := JobApplyStats{EntryCount: int64(len(batch.Entries))}
	for _, entry := range batch.Entries {
		if !entry.IsDir {
			stats.FileCount++
		}
	}
	return stats
}

func (s *JobApplyStats) add(other JobApplyStats) {
	if s == nil {
		return
	}
	s.EntryCount += other.EntryCount
	s.FileCount += other.FileCount
}
