package selection

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/keyboard"
)

var noSelection = errors.New("no selection")
var ErrSelectionUnsupported = errors.New("selection retrieval unsupported")
var lastClipboardChangeTimestamp atomic.Int64

var (
	internalSelectedTextMu sync.RWMutex
	internalSelectedText   func() (string, bool)
)

type SelectionType string

const (
	SelectionTypeText SelectionType = "text"
	SelectionTypeFile SelectionType = "file"
)

type Selection struct {
	Type SelectionType
	// Only available when Type is SelectionTypeText
	Text string
	// Only available when Type is SelectionTypeFile
	FilePaths []string
}

func InitSelection() {
	clipboard.Watch(func(data clipboard.Data) {
		lastClipboardChangeTimestamp.Store(util.GetSystemTimestamp())
	})
}

// SetInternalSelectedTextProvider registers in-process editor selection, used
// when a Wox window owns focus and OS UI Automation cannot see our custom UI.
func SetInternalSelectedTextProvider(provider func() (string, bool)) {
	internalSelectedTextMu.Lock()
	internalSelectedText = provider
	internalSelectedTextMu.Unlock()
}

// GetSelected prefers a focused Wox editor, then falls back to the OS capture path.
func GetSelected(ctx context.Context) (Selection, error) {
	if text, handled := lookupInternalSelectedText(); handled {
		if text == "" {
			return Selection{}, noSelection
		}
		util.GetLogger().Debug(ctx, fmt.Sprintf("using internal UI selected text, runes=%d", len([]rune(text))))
		return Selection{Type: SelectionTypeText, Text: text}, nil
	}
	return getSelectedFromOS(ctx)
}

func lookupInternalSelectedText() (string, bool) {
	internalSelectedTextMu.RLock()
	provider := internalSelectedText
	internalSelectedTextMu.RUnlock()
	if provider == nil {
		return "", false
	}
	return provider()
}

func (s *Selection) String() string {
	switch s.Type {
	case SelectionTypeText:
		return s.Text
	case SelectionTypeFile:
		return strings.Join(s.FilePaths, ";")
	}

	return ""
}

func (s *Selection) IsEmpty() bool {
	switch s.Type {
	case SelectionTypeText:
		return s.Text == ""
	case SelectionTypeFile:
		return s.FilePaths == nil || len(s.FilePaths) == 0
	}

	return false
}

const (
	clipboardCopyWait = 500 * time.Millisecond
	clipboardCopyPoll = 5 * time.Millisecond
)

func getSelectedByClipboard(ctx context.Context) (Selection, error) {
	startedAt := util.GetSystemTimestamp()
	seqBefore := clipboard.SequenceNumber()
	if keyboard.SimulateCopy() != nil {
		return Selection{}, errors.New("error simulate ctrl c")
	}
	simulateMs := util.GetSystemTimestamp() - startedAt

	clipboardDataAfter, err := waitForCopiedClipboard(seqBefore, startedAt)
	util.GetLogger().Debug(ctx, fmt.Sprintf("selection clipboard capture: simulate=%dms wait=%dms seqBefore=%d", simulateMs, util.GetSystemTimestamp()-startedAt-simulateMs, seqBefore))
	if err != nil {
		return Selection{}, err
	}

	switch clipboardDataAfter.GetType() {
	case clipboard.ClipboardTypeText:
		textData := clipboardDataAfter.(*clipboard.TextData)
		return Selection{
			Type: SelectionTypeText,
			Text: textData.Text,
		}, nil
	case clipboard.ClipboardTypeFile:
		fileData := clipboardDataAfter.(*clipboard.FilePathData)
		return Selection{
			Type:      SelectionTypeFile,
			FilePaths: fileData.FilePaths,
		}, nil
	}

	return Selection{}, errors.New("unknown clipboard type")
}

func waitForCopiedClipboard(seqBefore uint64, startedAt int64) (clipboard.Data, error) {
	deadline := time.Now().Add(clipboardCopyWait)
	for first := true; ; first = false {
		if !first {
			if !time.Now().Before(deadline) {
				return nil, noSelection
			}
			time.Sleep(clipboardCopyPoll)
		}
		if !clipboardCopyObserved(seqBefore, clipboard.SequenceNumber(), startedAt, lastClipboardChangeTimestamp.Load()) {
			continue
		}
		clipboardData, err := clipboard.ReadFilesAndText()
		if err != nil {
			continue
		}
		return clipboardData, nil
	}
}

func clipboardCopyObserved(seqBefore, seqNow uint64, startedAt, changedAt int64) bool {
	if seqBefore > 0 {
		return seqNow != seqBefore
	}
	return changedAt >= startedAt
}
