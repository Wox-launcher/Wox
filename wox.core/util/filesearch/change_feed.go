package filesearch

import (
	"context"
	"time"
)

type ChangeSignalKind string

const (
	ChangeSignalKindDirtyRoot             ChangeSignalKind = "dirty_root"
	ChangeSignalKindDirtyPath             ChangeSignalKind = "dirty_path"
	ChangeSignalKindRequiresRootReconcile ChangeSignalKind = "requires_root_reconcile"
	ChangeSignalKindFeedUnavailable       ChangeSignalKind = "feed_unavailable"
)

type ChangeSemanticKind string

const (
	ChangeSemanticKindUnknown               ChangeSemanticKind = "unknown"
	ChangeSemanticKindCreate                ChangeSemanticKind = "create"
	ChangeSemanticKindRemove                ChangeSemanticKind = "remove"
	ChangeSemanticKindRename                ChangeSemanticKind = "rename"
	ChangeSemanticKindModify                ChangeSemanticKind = "modify"
	ChangeSemanticKindMetadata              ChangeSemanticKind = "metadata"
	ChangeSemanticKindRequiresRootReconcile ChangeSemanticKind = "requires_root_reconcile"
	ChangeSemanticKindFeedUnavailable       ChangeSemanticKind = "feed_unavailable"
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
