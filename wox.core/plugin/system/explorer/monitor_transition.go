package explorer

import (
	"strconv"
	"time"
)

// ExplorerWindowRef identifies a file manager window that can be a Quick Switch source.
type ExplorerWindowRef struct {
	Pid      int
	WindowID string
}

// OpenSaveDialogActivatedEvent carries dialog identity and an optional direct Explorer/Finder source.
type OpenSaveDialogActivatedEvent struct {
	Pid              int
	WindowID         string
	PreviousExplorer *ExplorerWindowRef
}

type explorerForegroundKind int

const (
	explorerForegroundNone explorerForegroundKind = iota
	explorerForegroundExplorer
	explorerForegroundDialog
)

// ownerFocusStealWindow is how long an owned dialog may focus its owner Explorer
// before itself. Using that owner as the Quick Switch source jumps to the wrong folder.
const ownerFocusStealWindow = 400 * time.Millisecond

// explorerTransitionState records exact Explorer/Finder → Dialog transitions.
// Native monitors only report activation, deactivation, and window identity.
type explorerTransitionState struct {
	kind           explorerForegroundKind
	source         ExplorerWindowRef
	hasSource      bool
	previous       ExplorerWindowRef
	hasPrevious    bool
	lastExplorerAt time.Time
}

// ActivateExplorer records the current file manager window as the only valid Quick Switch source.
func (s *explorerTransitionState) ActivateExplorer(ref ExplorerWindowRef) {
	now := time.Now()
	if s.kind == explorerForegroundExplorer && s.hasSource && (s.source.Pid != ref.Pid || s.source.WindowID != ref.WindowID) {
		s.previous = s.source
		s.hasPrevious = true
	}
	s.kind = explorerForegroundExplorer
	s.source = ref
	s.hasSource = ref.Pid > 0
	s.lastExplorerAt = now
}

// ActivateDialog returns PreviousExplorer only when the previous foreground surface was Explorer/Finder.
func (s *explorerTransitionState) ActivateDialog(pid int, windowID string) OpenSaveDialogActivatedEvent {
	event := OpenSaveDialogActivatedEvent{
		Pid:      pid,
		WindowID: windowID,
	}
	if s.kind == explorerForegroundExplorer && s.hasSource {
		source := s.source
		// Clicking an owned Move Items dialog often focuses its owner Explorer first.
		if s.hasPrevious && time.Since(s.lastExplorerAt) < ownerFocusStealWindow {
			source = s.previous
		}
		event.PreviousExplorer = &source
	}
	s.kind = explorerForegroundDialog
	return event
}

// Deactivate clears the source immediately so a later dialog cannot guess a recent folder.
func (s *explorerTransitionState) Deactivate() {
	s.kind = explorerForegroundNone
	s.source = ExplorerWindowRef{}
	s.hasSource = false
	s.previous = ExplorerWindowRef{}
	s.hasPrevious = false
	s.lastExplorerAt = time.Time{}
}

// Reset returns the tracker to its initial empty state.
func (s *explorerTransitionState) Reset() {
	s.Deactivate()
}

func formatExplorerWindowID(windowID uintptr) string {
	if windowID == 0 {
		return ""
	}
	return strconv.FormatUint(uint64(windowID), 10)
}
