package filesearch

import (
	"path/filepath"
	"strings"
	"time"
)

const (
	usnReasonDataOverwrite    uint32 = 0x00000001
	usnReasonDataExtend       uint32 = 0x00000002
	usnReasonDataTruncation   uint32 = 0x00000004
	usnReasonNamedDataMask    uint32 = 0x00000070
	usnReasonFileCreate       uint32 = 0x00000100
	usnReasonFileDelete       uint32 = 0x00000200
	usnReasonEAChange         uint32 = 0x00000400
	usnReasonSecurityChange   uint32 = 0x00000800
	usnReasonRenameOldName    uint32 = 0x00001000
	usnReasonRenameNewName    uint32 = 0x00002000
	usnReasonIndexableChange  uint32 = 0x00004000
	usnReasonBasicInfoChange  uint32 = 0x00008000
	usnReasonHardLinkChange   uint32 = 0x00010000
	usnReasonCompressionMask  uint32 = 0x003e0000
	usnReasonObjectIDChange   uint32 = 0x00080000
	usnReasonReparseChange    uint32 = 0x00100000
	usnReasonIntegrityChange  uint32 = 0x00800000
	usnReasonTransactedChange uint32 = 0x00400000
)

type usnJournalState struct {
	Volume    string
	JournalID uint64
	FirstUSN  int64
	NextUSN   int64
}

type preparedUSNVolumeRefresh struct {
	roots    []RootRecord
	startUSN int64
	signals  []ChangeSignal
}

func prepareUSNVolumeRefresh(roots []RootRecord, journal usnJournalState, now time.Time, safeWindow time.Duration) preparedUSNVolumeRefresh {
	prepared := preparedUSNVolumeRefresh{
		roots:    append([]RootRecord(nil), roots...),
		startUSN: journal.NextUSN,
	}

	haveFreshCursor := false
	for _, root := range roots {
		if root.FeedState == RootFeedStateUnavailable {
			prepared.signals = append(prepared.signals, newUSNRecoverySignal(root, "usn feed recovered", now))
			continue
		}

		cursor, ok := decodeFeedCursor(root.FeedCursor, RootFeedTypeUSN)
		if !ok {
			if root.FeedCursor != "" {
				prepared.signals = append(prepared.signals, newUSNRecoverySignal(root, "invalid usn cursor", now))
			}
			continue
		}
		if !feedCursorFresh(cursor, now, safeWindow) {
			prepared.signals = append(prepared.signals, newUSNRecoverySignal(root, "expired usn cursor", now))
			continue
		}
		if cursor.JournalID != journal.JournalID {
			prepared.signals = append(prepared.signals, newUSNRecoverySignal(root, "stale usn journal id", now))
			continue
		}
		if cursor.Volume != "" && !strings.EqualFold(cursor.Volume, journal.Volume) {
			prepared.signals = append(prepared.signals, newUSNRecoverySignal(root, "usn cursor volume changed", now))
			continue
		}
		if cursor.USN < journal.FirstUSN || cursor.USN > journal.NextUSN {
			prepared.signals = append(prepared.signals, newUSNRecoverySignal(root, "usn cursor outside journal retention window", now))
			continue
		}

		if !haveFreshCursor || cursor.USN < prepared.startUSN {
			prepared.startUSN = cursor.USN
			haveFreshCursor = true
		}
	}

	if !haveFreshCursor && prepared.startUSN < 0 {
		prepared.startUSN = 0
	}

	return prepared
}

func translateUSNDelta(root RootRecord, journal usnJournalState, path string, pathIsDir bool, pathTypeKnown bool, usn int64, reason uint32, at time.Time) ChangeSignal {
	cleanPath := filepath.Clean(path)
	if cleanPath == "." || cleanPath == "" {
		cleanPath = filepath.Clean(root.Path)
	}

	cursorText := ""
	if usn > 0 {
		cursor, err := encodeFeedCursor(FeedCursor{
			FeedType:  RootFeedTypeUSN,
			UpdatedAt: at.UnixMilli(),
			JournalID: journal.JournalID,
			USN:       usn,
			Volume:    journal.Volume,
		})
		if err == nil {
			cursorText = cursor
		}
	}

	kind := ChangeSignalKindDirtyPath
	if cleanPath == filepath.Clean(root.Path) {
		kind = ChangeSignalKindDirtyRoot
		pathIsDir = true
		pathTypeKnown = true
	}

	return ChangeSignal{
		Kind:          kind,
		SemanticKind:  classifyUSNSemanticKind(reason),
		RootID:        root.ID,
		FeedType:      RootFeedTypeUSN,
		Path:          cleanPath,
		PathIsDir:     pathIsDir,
		PathTypeKnown: pathTypeKnown,
		Cursor:        cursorText,
		At:            at,
	}
}

func classifyUSNSemanticKind(reason uint32) ChangeSemanticKind {
	// Feature change: app indexing consumes the shared change feed directly and
	// must distinguish real app-file changes from metadata noise. USN exposes a
	// bitmask instead of fsnotify operations, so normalize it once at the feed
	// boundary and let consumers drop untrusted or irrelevant events cheaply.
	switch {
	case reason&(usnReasonRenameOldName|usnReasonRenameNewName) != 0:
		return ChangeSemanticKindRename
	case reason&usnReasonFileCreate != 0:
		return ChangeSemanticKindCreate
	case reason&usnReasonFileDelete != 0:
		return ChangeSemanticKindRemove
	case reason&(usnReasonDataOverwrite|usnReasonDataExtend|usnReasonDataTruncation|usnReasonNamedDataMask) != 0:
		return ChangeSemanticKindModify
	case reason&(usnReasonEAChange|usnReasonSecurityChange|usnReasonIndexableChange|usnReasonBasicInfoChange|usnReasonHardLinkChange|usnReasonCompressionMask|usnReasonObjectIDChange|usnReasonReparseChange|usnReasonIntegrityChange|usnReasonTransactedChange) != 0:
		return ChangeSemanticKindMetadata
	default:
		return ChangeSemanticKindUnknown
	}
}

func newUSNRecoverySignal(root RootRecord, reason string, at time.Time) ChangeSignal {
	return ChangeSignal{
		Kind:          ChangeSignalKindRequiresRootReconcile,
		RootID:        root.ID,
		FeedType:      RootFeedTypeUSN,
		Path:          root.Path,
		PathIsDir:     true,
		PathTypeKnown: true,
		Reason:        reason,
		At:            at,
	}
}
