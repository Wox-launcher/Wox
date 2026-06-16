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
	if eventPath == "" {
		eventPath = root.Path
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

	if fseventRequiresRootReconcile(flags) {
		return []ChangeSignal{{
			Kind:          ChangeSignalKindRequiresRootReconcile,
			SemanticKind:  ChangeSemanticKindRequiresRootReconcile,
			RootID:        root.ID,
			FeedType:      RootFeedTypeFSEvents,
			Path:          root.Path,
			PathIsDir:     true,
			PathTypeKnown: true,
			Reason:        "fsevents flagged history loss or root change",
			Cursor:        cursorText,
			At:            at,
		}}
	}

	pathIsDir := flags&fseventFlagItemIsDir != 0
	pathTypeKnown := pathIsDir || flags&fseventFlagItemIsFile != 0
	kind := ChangeSignalKindDirtyPath
	semanticKind := translateFSEventSemanticKind(flags)
	if eventPath == filepath.Clean(root.Path) {
		kind = ChangeSignalKindDirtyRoot
		pathIsDir = true
		pathTypeKnown = true
	}

	return []ChangeSignal{{
		Kind:          kind,
		SemanticKind:  semanticKind,
		RootID:        root.ID,
		FeedType:      RootFeedTypeFSEvents,
		Path:          eventPath,
		PathIsDir:     pathIsDir,
		PathTypeKnown: pathTypeKnown,
		Cursor:        cursorText,
		At:            at,
	}}
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

func fseventRequiresRootReconcile(flags uint64) bool {
	return flags&(fseventFlagMustScanSubDirs|fseventFlagUserDropped|fseventFlagKernelDropped|fseventFlagEventIDsWrapped|fseventFlagRootChanged|fseventFlagMount|fseventFlagUnmount) != 0
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
