package filesearch

import (
	"context"
	"time"
)

type ChangeSignalKind string

const (
	ChangeSignalKindDirtyPath ChangeSignalKind = "dirty_path"
	// ChangeSignalKindHistoryLost means the feed can no longer prove it reported
	// every change under the root since its last acknowledged cursor: a full signal
	// queue, a kernel queue overflow, an expired or reset journal, or a feed that
	// just recovered. Every platform reports loss with this one kind, and the
	// scanner repairs it with a directory mtime relist instead of re-crawling.
	ChangeSignalKindHistoryLost     ChangeSignalKind = "history_lost"
	ChangeSignalKindFeedUnavailable ChangeSignalKind = "feed_unavailable"
)

type ChangeSemanticKind string

const (
	ChangeSemanticKindUnknown         ChangeSemanticKind = "unknown"
	ChangeSemanticKindCreate          ChangeSemanticKind = "create"
	ChangeSemanticKindRemove          ChangeSemanticKind = "remove"
	ChangeSemanticKindRename          ChangeSemanticKind = "rename"
	ChangeSemanticKindModify          ChangeSemanticKind = "modify"
	ChangeSemanticKindMetadata        ChangeSemanticKind = "metadata"
	ChangeSemanticKindHistoryLost     ChangeSemanticKind = "history_lost"
	ChangeSemanticKindFeedUnavailable ChangeSemanticKind = "feed_unavailable"
)

type ChangeSignal struct {
	Kind          ChangeSignalKind
	SemanticKind  ChangeSemanticKind
	RootID        string
	FeedType      RootFeedType
	Path          string
	PathIsDir     bool
	PathTypeKnown bool
	Reason        string
	Cursor        string
	At            time.Time
}

type ChangeFeed interface {
	Mode() string
	Signals() <-chan ChangeSignal
	Refresh(ctx context.Context, roots []RootRecord) error
	Close() error
}

func NewChangeFeed() ChangeFeed {
	// Feature change: non-file-search consumers need the same platform change
	// source without depending on Scanner internals. Returning the existing
	// platform feed lets app indexing reuse USN/FSEvents precision instead of
	// adding recursive fsnotify watchers that fail on large directory trees.
	return newPlatformChangeFeed()
}

type RootFeedSnapshot struct {
	FeedType   RootFeedType
	FeedCursor string
	FeedState  RootFeedState
}

type RootFeedSnapshotter interface {
	SnapshotRootFeed(ctx context.Context, root RootRecord) (RootFeedSnapshot, error)
}

// RootFeedAcknowledger updates replay boundaries only after durable indexing.
type RootFeedAcknowledger interface {
	AcknowledgeRoot(root RootRecord)
}

// DirectoryWatchFilter reports whether a feed should register a native watch
// for directory under root.
type DirectoryWatchFilter func(root RootRecord, directory string) bool

// DirectoryWatchFilterSetter is implemented by feeds that must register one
// native watch per directory (fsnotify). The scanner installs its ignore policy
// so excluded trees such as node_modules do not consume the watch budget.
type DirectoryWatchFilterSetter interface {
	SetDirectoryWatchFilter(filter DirectoryWatchFilter)
}
