package explorer

import "strconv"

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

// explorerTransitionState records exact Explorer/Finder → Dialog transitions.
// Native monitors only report activation, deactivation, and window identity.
type explorerTransitionState struct {
	kind      explorerForegroundKind
	source    ExplorerWindowRef
	hasSource bool
}

// ActivateExplorer records the current file manager window as the only valid Quick Switch source.
func (s *explorerTransitionState) ActivateExplorer(ref ExplorerWindowRef) {
	s.kind = explorerForegroundExplorer
	s.source = ref
	s.hasSource = ref.Pid > 0
}

// ActivateDialog returns PreviousExplorer only when the previous foreground surface was Explorer/Finder.
func (s *explorerTransitionState) ActivateDialog(pid int, windowID string) OpenSaveDialogActivatedEvent {
	event := OpenSaveDialogActivatedEvent{
		Pid:      pid,
		WindowID: windowID,
	}
	if s.kind == explorerForegroundExplorer && s.hasSource {
		source := s.source
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
