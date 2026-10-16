//go:build darwin

package filesearch

import (
	"path/filepath"
	"time"
)

const (
	fseventSinceNow             uint64 = ^uint64(0)
	fseventFlagMustScanSubDirs         = 0x00000001
	fseventFlagUserDropped             = 0x00000002
	fseventFlagKernelDropped           = 0x00000004
	fseventFlagEventIDsWrapped         = 0x00000008
	fseventFlagHistoryDone             = 0x00000010
	fseventFlagRootChanged             = 0x00000020
	fseventFlagMount                   = 0x00000040
	fseventFlagUnmount                 = 0x00000080
	fseventFlagItemCreated             = 0x00000100
	fseventFlagItemRemoved             = 0x00000200
	fseventFlagItemInodeMetaMod        = 0x00000400
	fseventFlagItemRenamed             = 0x00000800
	fseventFlagItemModified            = 0x00001000
	fseventFlagItemIsFile              = 0x00010000
	fseventFlagItemIsDir               = 0x00020000
)

type preparedFSEventsRefresh struct {
	watchRoots   []RootRecord
	sinceEventID uint64
	signals      []ChangeSignal
}

func prepareFSEventsRefresh(roots []RootRecord, now time.Time, safeWindow time.Duration) preparedFSEventsRefresh {
	prepared := preparedFSEventsRefresh{
		watchRoots:   append([]RootRecord(nil), roots...),
		sinceEventID: fseventSinceNow,
	}

	haveFreshCursor := false
	for _, root := range roots {
		if root.FeedState == RootFeedStateUnavailable {
			prepared.signals = append(prepared.signals, newFSEventsRecoverySignal(root, "fsevents feed recovered", now))
		}

		cursor, ok := decodeFeedCursor(root.FeedCursor, RootFeedTypeFSEvents)
		if !ok {
			if root.FeedCursor != "" {
				prepared.signals = append(prepared.signals, newFSEventsRecoverySignal(root, "invalid fsevents cursor", now))
			}
			continue
		}
		if !feedCursorFresh(cursor, now, safeWindow) {
			prepared.signals = append(prepared.signals, newFSEventsRecoverySignal(root, "expired fsevents cursor", now))
			continue
		}

		if !haveFreshCursor || cursor.FSEventID < prepared.sinceEventID {
			prepared.sinceEventID = cursor.FSEventID
			haveFreshCursor = true
		}
	}

	if !haveFreshCursor {
		prepared.sinceEventID = fseventSinceNow
	}

	return prepared
}

func translateFSEvent(root RootRecord, eventPath string, flags uint64, eventID uint64, at time.Time) []ChangeSignal {
	eventPath = filepath.Clean(eventPath)
	rootPath := filepath.Clean(root.Path)
	if eventPath == "" {
		eventPath = rootPath
	}

	// A shared stream starts at the oldest root cursor. Other roots must not
	// requeue history they have already durably applied. Identity changes are
	// still handled even when event numbering has reset.
	if cursor, ok := decodeFeedCursor(root.FeedCursor, RootFeedTypeFSEvents); ok && eventID > 0 && eventID <= cursor.FSEventID && !fseventHistoryUnusable(flags) {
		return nil
	}

	cursorText := ""
	if eventID > 0 {
		cursor, err := encodeFeedCursor(FeedCursor{
			FeedType:  RootFeedTypeFSEvents,
			UpdatedAt: at.UnixMilli(),
			FSEventID: eventID,
		})
		if err == nil {
			cursorText = cursor
		}
	}

	// HistoryDone only marks the end of a replay. Item flags on the same event
	// still describe a real change.
	flags &^= fseventFlagHistoryDone
	if flags == 0 {
		return nil
	}

	eventIsRoot := eventPath == rootPath
	// MustScanSubDirs means detailed history is unavailable. Replaying the
	// same cursor can return the same loss marker forever; reconcile once
	// through the scanner's coalescing/cooldown gate instead.
	if eventIsRoot && fseventRootHistoryLost(flags) {
		return []ChangeSignal{newFSEventsRecoverySignal(root, "fsevents root history lost", at)}
	}
	// Mount identity and wrapped event ids cannot be recovered by replaying
	// the previous cursor. Keep a root reconcile only when the event names
	// the configured root itself.
	if eventIsRoot && fseventHistoryUnusable(flags) {
		return []ChangeSignal{newFSEventsRecoverySignal(root, "fsevents root history is unusable", at)}
	}
	// Must-scan and drop flags name one directory. Rescan that directory,
	// not every configured root.
	if !eventIsRoot && fseventNeedsDirectoryRescan(flags) {
		return []ChangeSignal{newFSEventsDirectorySignal(root, eventPath, cursorText, at)}
	}
	// File-level events already report children of the watch root. A plain
	// event whose path is the root is the directory inode itself; queuing it
	// as a dirty parent would collapse every other path in the batch into a
	// whole-root walk.
	if eventIsRoot {
		return nil
	}

	pathIsDir := flags&fseventFlagItemIsDir != 0
	pathTypeKnown := pathIsDir || flags&fseventFlagItemIsFile != 0
	return []ChangeSignal{{
		Kind:          ChangeSignalKindDirtyPath,
		SemanticKind:  translateFSEventSemanticKind(flags),
		RootID:        root.ID,
		FeedType:      RootFeedTypeFSEvents,
		Path:          eventPath,
		PathIsDir:     pathIsDir,
		PathTypeKnown: pathTypeKnown,
		Cursor:        cursorText,
		At:            at,
	}}
}

// fseventRootHistoryLost identifies loss markers requiring a root reconcile.
func fseventRootHistoryLost(flags uint64) bool {
	if flags&(fseventFlagMustScanSubDirs|fseventFlagUserDropped|fseventFlagKernelDropped) == 0 {
		return false
	}
	return !fseventHistoryUnusable(flags)
}

func fseventHistoryUnusable(flags uint64) bool {
	return flags&(fseventFlagEventIDsWrapped|fseventFlagRootChanged|fseventFlagMount|fseventFlagUnmount) != 0
}

func fseventNeedsDirectoryRescan(flags uint64) bool {
	return flags&(fseventFlagMustScanSubDirs|fseventFlagUserDropped|fseventFlagKernelDropped|fseventFlagEventIDsWrapped|fseventFlagRootChanged|fseventFlagMount|fseventFlagUnmount) != 0
}

func newFSEventsDirectorySignal(root RootRecord, path string, cursorText string, at time.Time) ChangeSignal {
	return ChangeSignal{
		Kind:          ChangeSignalKindDirtyPath,
		SemanticKind:  ChangeSemanticKindUnknown,
		RootID:        root.ID,
		FeedType:      RootFeedTypeFSEvents,
		Path:          path,
		PathIsDir:     true,
		PathTypeKnown: true,
		Reason:        "fsevents directory needs a scoped rescan",
		Cursor:        cursorText,
		At:            at,
	}
}

func translateFSEventSemanticKind(flags uint64) ChangeSemanticKind {
	switch {
	case flags&fseventFlagItemRenamed != 0:
		return ChangeSemanticKindRename
	case flags&fseventFlagItemRemoved != 0:
		return ChangeSemanticKindRemove
	case flags&fseventFlagItemCreated != 0:
		return ChangeSemanticKindCreate
	case flags&fseventFlagItemModified != 0:
		return ChangeSemanticKindModify
	case flags&fseventFlagItemInodeMetaMod != 0:
		return ChangeSemanticKindMetadata
	default:
		return ChangeSemanticKindUnknown
	}
}

func sameFSEventsWatchRoots(left []RootRecord, right []RootRecord) bool {
	if len(left) != len(right) {
		return false
	}

	type watchKey struct {
		id   string
		path string
	}
	seen := make(map[watchKey]struct{}, len(left))
	for _, root := range left {
		seen[watchKey{id: root.ID, path: filepath.Clean(root.Path)}] = struct{}{}
	}
	for _, root := range right {
		if _, ok := seen[watchKey{id: root.ID, path: filepath.Clean(root.Path)}]; !ok {
			return false
		}
	}
	return true
}

func newFSEventsRecoverySignal(root RootRecord, reason string, at time.Time) ChangeSignal {
	return ChangeSignal{
		Kind:          ChangeSignalKindRequiresRootReconcile,
		SemanticKind:  ChangeSemanticKindRequiresRootReconcile,
		RootID:        root.ID,
		FeedType:      RootFeedTypeFSEvents,
		Path:          root.Path,
		PathIsDir:     true,
		PathTypeKnown: true,
		Reason:        reason,
		At:            at,
	}
}
